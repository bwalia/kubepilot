#!/usr/bin/env ruby
# frozen_string_literal: true

# Publish App Privacy nutrition labels as "Data Not Collected".
# Tries the public Connect API first (JWT), then ASC iris (session/JWT).
#
# Matches docs/landing/privacy.html: no developer analytics/ads; credentials
# stay on device; ops data goes to the user-configured KubePilot server.
#
# Requires: ASC_KEY_ID, ASC_ISSUER_ID, ASC_KEY_FILEPATH
# Optional: ASC_APP_ID (default 6791872512)

require "json"
require "jwt"
require "net/http"
require "openssl"
require "uri"

APP_ID = ENV.fetch("ASC_APP_ID", "6791872512")
API = "https://api.appstoreconnect.apple.com/v1"
IRIS = "https://appstoreconnect.apple.com/iris/v1"

def mint_token
  key = OpenSSL::PKey.read(File.read(ENV.fetch("ASC_KEY_FILEPATH")))
  now = Time.now.to_i
  JWT.encode(
    {
      iss: ENV.fetch("ASC_ISSUER_ID"),
      iat: now,
      exp: now + 20 * 60,
      aud: "appstoreconnect-v1"
    },
    key,
    "ES256",
    { kid: ENV.fetch("ASC_KEY_ID"), typ: "JWT" }
  )
end

def http_json(base, method, path, token, body = nil)
  uri = URI("#{base}/#{path.sub(%r{\A/}, '')}")
  http = Net::HTTP.new(uri.host, uri.port)
  http.use_ssl = true
  http.open_timeout = 30
  http.read_timeout = 120
  klass = {
    get: Net::HTTP::Get,
    post: Net::HTTP::Post,
    patch: Net::HTTP::Patch,
    delete: Net::HTTP::Delete
  }.fetch(method)
  req = klass.new(uri)
  req["Authorization"] = "Bearer #{token}"
  req["Content-Type"] = "application/json"
  req["Accept"] = "application/json"
  req.body = JSON.generate(body) if body
  res = http.request(req)
  parsed = res.body.to_s.empty? ? nil : (JSON.parse(res.body) rescue { "raw" => res.body.to_s[0, 800] })
  [res.code.to_i, parsed]
end

def create_not_collected!(base, token)
  http_json(
    base,
    :post,
    "appDataUsages",
    token,
    {
      data: {
        type: "appDataUsages",
        relationships: {
          app: { data: { type: "apps", id: APP_ID } },
          dataProtection: {
            data: { type: "appDataUsageDataProtections", id: "DATA_NOT_COLLECTED" }
          }
        }
      }
    }
  )
end

def publish!(base, token)
  code, payload = http_json(base, :get, "apps/#{APP_ID}/dataUsagePublishState", token)
  return [code, payload] unless code.between?(200, 299)

  state_id = payload.dig("data", "id")
  return [200, payload] if payload.dig("data", "attributes", "published")
  return [422, { "error" => "missing publish state id" }] unless state_id

  http_json(
    base,
    :patch,
    "appDataUsagesPublishState/#{state_id}",
    token,
    { data: { type: "appDataUsagesPublishState", id: state_id, attributes: { published: true } } }
  )
end

def list_usages(base, token)
  code, payload = http_json(
    base,
    :get,
    "apps/#{APP_ID}/dataUsages?include=dataProtection&limit=200",
    token
  )
  return [] unless code.between?(200, 299)

  payload.fetch("data")
end

token = mint_token
puts "Ensuring App Privacy DATA_NOT_COLLECTED for app #{APP_ID}…"

ok = false
errors = []

[API, IRIS].each do |base|
  begin
    usages = list_usages(base, token)
    already = usages.any? do |u|
      u.dig("relationships", "dataProtection", "data", "id") == "DATA_NOT_COLLECTED"
    end

    unless already
      # Best-effort cleanup of prior declarations when list works.
      usages.each do |u|
        http_json(base, :delete, "appDataUsages/#{u.fetch('id')}", token)
      end

      code, payload = create_not_collected!(base, token)
      if code.between?(200, 299)
        puts "Created DATA_NOT_COLLECTED via #{base}"
      elsif code == 409
        puts "DATA_NOT_COLLECTED already exists via #{base} (409)"
      else
        errors << "create #{base}: #{code} #{payload}"
        next
      end
    else
      puts "DATA_NOT_COLLECTED already present via #{base}"
    end

    code, payload = publish!(base, token)
    if code.between?(200, 299)
      puts "App Privacy publish ok via #{base}"
      ok = true
      break
    end
    errors << "publish #{base}: #{code} #{payload}"
  rescue StandardError => e
    errors << "#{base}: #{e.message}"
  end
end

unless ok
  warn "App Privacy automation failed (ASC API key cannot publish iris nutrition labels)."
  warn errors.join("\n")
  warn "One-time ASC UI step: App Privacy → Data Not Collected → Publish, then re-run app_store."
  # Soft-skip so deliver can still upload age rating / screenshots / metadata.
  # Submit-for-review will keep failing until privacy is published in ASC.
  exit 0
end

puts "Done."
