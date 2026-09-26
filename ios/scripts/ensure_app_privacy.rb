#!/usr/bin/env ruby
# frozen_string_literal: true

# Publish App Privacy nutrition labels as "Data Not Collected" via ASC iris
# (Spaceship's public ConnectAPI client no longer exposes apps.dataUsages).
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

def http_json(method, path, token, body = nil)
  uri = URI("#{IRIS}/#{path.sub(%r{\A/}, '')}")
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
  parsed = res.body.to_s.empty? ? nil : (JSON.parse(res.body) rescue { "raw" => res.body.to_s[0, 500] })
  [res.code.to_i, parsed]
end

def list_usages(token)
  items = []
  path = "apps/#{APP_ID}/dataUsages?include=category,purpose,dataProtection&limit=200"
  loop do
    code, payload = http_json(:get, path, token)
    raise "List dataUsages failed (#{code}): #{payload}" unless code.between?(200, 299)

    items.concat(payload.fetch("data"))
    next_link = payload.dig("links", "next")
    break unless next_link

    path = next_link.sub("#{IRIS}/", "")
  end
  items
end

def not_collected?(usage, included)
  prot_id = usage.dig("relationships", "dataProtection", "data", "id")
  return prot_id == "DATA_NOT_COLLECTED" if prot_id

  false
end

token = mint_token
puts "Ensuring App Privacy DATA_NOT_COLLECTED for app #{APP_ID}…"

usages = list_usages(token)
existing = usages.select { |u| not_collected?(u, nil) }

if existing.any?
  puts "DATA_NOT_COLLECTED already present (#{existing.size})"
else
  usages.each do |usage|
    code, payload = http_json(:delete, "appDataUsages/#{usage.fetch('id')}", token)
    if code.between?(200, 299) || code == 204
      puts "Removed prior data usage #{usage.fetch('id')}"
    else
      warn "Could not delete #{usage.fetch('id')} (#{code}): #{payload}"
    end
  end

  code, payload = http_json(
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
  raise "Create DATA_NOT_COLLECTED failed (#{code}): #{payload}" unless code.between?(200, 299)

  puts "Created DATA_NOT_COLLECTED App Privacy declaration"
end

code, payload = http_json(:get, "apps/#{APP_ID}/dataUsagePublishState", token)
raise "Get publish state failed (#{code}): #{payload}" unless code.between?(200, 299)

state_id = payload.dig("data", "id")
published = payload.dig("data", "attributes", "published")
if published
  puts "App Privacy already published"
else
  raise "Missing publish state id" unless state_id

  code, payload = http_json(
    :patch,
    "appDataUsagesPublishState/#{state_id}",
    token,
    { data: { type: "appDataUsagesPublishState", id: state_id, attributes: { published: true } } }
  )
  raise "Publish App Privacy failed (#{code}): #{payload}" unless code.between?(200, 299)

  puts "Published App Privacy answers"
end

puts "Done."
