#!/usr/bin/env ruby
# frozen_string_literal: true

# Bootstrap free worldwide pricing + territory availability via the current
# App Store Connect APIs. fastlane deliver's price_tier still hits the removed
# apps.prices / availableTerritories relationships and 400s — set pricing here
# once (idempotent), then deliver without price_tier.
#
# Requires: ASC_KEY_ID, ASC_ISSUER_ID, ASC_KEY_FILEPATH
# Optional: ASC_APP_ID (default 6791872512), ASC_BASE_TERRITORY (default GBR)

require "json"
require "jwt"
require "net/http"
require "openssl"
require "uri"

APP_ID = ENV.fetch("ASC_APP_ID", "6791872512")
BASE_TERRITORY = ENV.fetch("ASC_BASE_TERRITORY", "GBR")
API = "https://api.appstoreconnect.apple.com"

def mint_token
  key_id = ENV.fetch("ASC_KEY_ID")
  issuer = ENV.fetch("ASC_ISSUER_ID")
  key = OpenSSL::PKey.read(File.read(ENV.fetch("ASC_KEY_FILEPATH")))
  now = Time.now.to_i
  JWT.encode(
    { iss: issuer, iat: now, exp: now + 20 * 60, aud: "appstoreconnect-v1" },
    key,
    "ES256",
    { kid: key_id, typ: "JWT" }
  )
end

def request(method, path, body = nil)
  uri = URI.join(API + "/", path.sub(%r{\A/}, ""))
  http = Net::HTTP.new(uri.host, uri.port)
  http.use_ssl = true
  http.open_timeout = 30
  http.read_timeout = 120

  klass = { get: Net::HTTP::Get, post: Net::HTTP::Post, patch: Net::HTTP::Patch }.fetch(method)
  req = klass.new(uri)
  req["Authorization"] = "Bearer #{@token}"
  req["Content-Type"] = "application/json"
  req.body = JSON.generate(body) if body

  res = http.request(req)
  parsed = res.body.to_s.empty? ? nil : JSON.parse(res.body)
  [res.code.to_i, parsed]
end

def get_paginated(path)
  items = []
  next_path = path
  while next_path
    code, payload = request(:get, next_path)
    raise "GET #{next_path} failed (#{code}): #{payload}" unless code.between?(200, 299)

    items.concat(payload.fetch("data"))
    next_link = payload.dig("links", "next")
    next_path = next_link ? next_link.sub(API, "") : nil
  end
  items
end

def free_price_point_id
  path = "v1/apps/#{APP_ID}/appPricePoints?filter[territory]=#{BASE_TERRITORY}&limit=200"
  points = get_paginated(path)
  free = points.find do |p|
    price = p.dig("attributes", "customerPrice").to_s
    price == "0" || price == "0.0" || price == "0.00"
  end
  raise "No free (0.00) price point for #{BASE_TERRITORY}" unless free

  free.fetch("id")
end

def schedule_has_current_price?(schedule_id)
  return false if schedule_id.to_s.empty?

  code, payload = request(
    :get,
    "v1/appPriceSchedules/#{schedule_id}/manualPrices?include=appPricePoint&limit=50"
  )
  return false unless code.between?(200, 299)

  (payload["data"] || []).any? do |price|
    # Current price has null startDate (live immediately) or a past start.
    attrs = price["attributes"] || {}
    start = attrs["startDate"]
    start.nil? || start.to_s <= Time.now.utc.strftime("%Y-%m-%d")
  end
end

def ensure_price_schedule!
  schedule_id = nil
  code, payload = request(:get, "v1/apps/#{APP_ID}/appPriceSchedule")
  schedule_id = payload.dig("data", "id") if code.between?(200, 299)

  if schedule_has_current_price?(schedule_id)
    puts "Free price schedule already active (#{schedule_id}) — skipping"
    return
  end

  # Relationship stub can exist without any manualPrices; POST creates/replaces
  # the schedule with a free base-territory price (Apple equalizes the rest).
  point_id = free_price_point_id
  body = {
    data: {
      type: "appPriceSchedules",
      relationships: {
        app: { data: { type: "apps", id: APP_ID } },
        baseTerritory: { data: { type: "territories", id: BASE_TERRITORY } },
        manualPrices: {
          data: [{ type: "appPrices", id: "${price-0}" }]
        }
      }
    },
    included: [
      {
        type: "appPrices",
        id: "${price-0}",
        attributes: { startDate: nil },
        relationships: {
          appPricePoint: { data: { type: "appPricePoints", id: point_id } }
        }
      }
    ]
  }

  code, payload = request(:post, "v1/appPriceSchedules", body)
  raise "Create price schedule failed (#{code}): #{payload}" unless code.between?(200, 299)

  puts "Created free price schedule (base=#{BASE_TERRITORY}, point=#{point_id})"
end

def all_territory_ids
  get_paginated("v1/territories?limit=200").map { |t| t.fetch("id") }
end

def ensure_worldwide_availability!
  code, payload = request(
    :get,
    "v2/appAvailabilities/#{APP_ID}/territoryAvailabilities?limit=200&filter[available]=true"
  )
  if code.between?(200, 299)
    available = []
    next_path = "v2/appAvailabilities/#{APP_ID}/territoryAvailabilities?limit=200&filter[available]=true"
    while next_path
      c, p = request(:get, next_path)
      break unless c.between?(200, 299)

      available.concat(p.fetch("data"))
      next_link = p.dig("links", "next")
      next_path = next_link ? next_link.sub(API, "") : nil
    end
    if available.size >= 150
      puts "Worldwide availability already set (#{available.size} territories) — skipping"
      return
    end
  end

  territories = all_territory_ids
  included = territories.map do |code|
    {
      type: "territoryAvailabilities",
      id: "${#{code}}",
      attributes: { available: true },
      relationships: {
        territory: { data: { type: "territories", id: code } }
      }
    }
  end

  body = {
    data: {
      type: "appAvailabilities",
      attributes: { availableInNewTerritories: true },
      relationships: {
        app: { data: { type: "apps", id: APP_ID } },
        territoryAvailabilities: {
          data: territories.map { |code| { type: "territoryAvailabilities", id: "${#{code}}" } }
        }
      }
    },
    included: included
  }

  code, payload = request(:post, "v2/appAvailabilities", body)
  raise "Create app availability failed (#{code}): #{payload}" unless code.between?(200, 299)

  puts "Enabled availability in #{territories.size} territories (availableInNewTerritories=true)"
end

@token = mint_token
puts "Ensuring free worldwide pricing for app #{APP_ID}…"
ensure_price_schedule!
ensure_worldwide_availability!
puts "Done."
