#!/usr/bin/env ruby
# frozen_string_literal: true

# Publish App Privacy nutrition labels as "Data Not Collected".
# Matches docs/landing/privacy.html: the iOS app has no developer-operated
# analytics/ads backend; credentials stay on device and ops data goes to the
# user-configured KubePilot server.
#
# Requires: ASC_KEY_ID, ASC_ISSUER_ID, ASC_KEY_FILEPATH
# Optional: ASC_APP_ID (default 6791872512)

require "json"
require "jwt"
require "net/http"
require "openssl"
require "spaceship"
require "uri"

APP_ID = ENV.fetch("ASC_APP_ID", "6791872512")
IRIS = "https://appstoreconnect.apple.com/iris/v1"
API = "https://api.appstoreconnect.apple.com/v1"

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

def http_json(method, url, token, body = nil)
  uri = URI(url)
  http = Net::HTTP.new(uri.host, uri.port)
  http.use_ssl = true
  http.open_timeout = 30
  http.read_timeout = 120
  klass = { get: Net::HTTP::Get, post: Net::HTTP::Post, patch: Net::HTTP::Patch }.fetch(method)
  req = klass.new(uri)
  req["Authorization"] = "Bearer #{token}"
  req["Content-Type"] = "application/json"
  req["Accept"] = "application/json"
  req.body = JSON.generate(body) if body
  res = http.request(req)
  parsed = res.body.to_s.empty? ? nil : (JSON.parse(res.body) rescue { "raw" => res.body })
  [res.code.to_i, parsed]
end

def publish_privacy!(token)
  [[API, "apps/#{APP_ID}/dataUsagePublishState", "appDataUsagesPublishState"],
   [IRIS, "apps/#{APP_ID}/dataUsagePublishState", "appDataUsagesPublishState"]].each do |base, get_path, type|
    code, payload = http_json(:get, "#{base}/#{get_path}", token)
    next unless code.between?(200, 299)

    state_id = payload.dig("data", "id")
    published = payload.dig("data", "attributes", "published")
    if published
      puts "App Privacy already published (#{base})"
      return true
    end
    next unless state_id

    patch_code, patch_payload = http_json(
      :patch,
      "#{base}/#{type}/#{state_id}",
      token,
      { data: { type: type, id: state_id, attributes: { published: true } } }
    )
    if patch_code.between?(200, 299)
      puts "Published App Privacy answers via #{base}"
      return true
    end
    warn "Publish via #{base} failed (#{patch_code}): #{patch_payload}"
  end
  false
end

Spaceship::ConnectAPI.token = Spaceship::ConnectAPI::Token.create(
  key_id: ENV.fetch("ASC_KEY_ID"),
  issuer_id: ENV.fetch("ASC_ISSUER_ID"),
  filepath: ENV.fetch("ASC_KEY_FILEPATH")
)
jwt = mint_token

usages = Spaceship::ConnectAPI::AppDataUsage.all(
  app_id: APP_ID,
  includes: "dataProtection,category,purpose",
  limit: 200
)

not_collected = usages.select(&:is_not_collected?)
if not_collected.any?
  puts "App Privacy already declares DATA_NOT_COLLECTED (#{not_collected.size})"
else
  usages.each do |usage|
    begin
      usage.delete!
      puts "Removed prior data usage #{usage.id}"
    rescue StandardError => e
      warn "Could not delete usage #{usage.id}: #{e.message}"
    end
  end

  Spaceship::ConnectAPI::AppDataUsage.create(
    app_id: APP_ID,
    app_data_usage_protection_id: Spaceship::ConnectAPI::AppDataUsageDataProtection::ID::DATA_NOT_COLLECTED
  )
  puts "Created DATA_NOT_COLLECTED App Privacy declaration"
end

unless publish_privacy!(jwt)
  warn "Could not confirm App Privacy publish — submit may still fail if ASC requires an explicit publish"
end

puts "Done."
