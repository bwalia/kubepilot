#!/usr/bin/env ruby
# frozen_string_literal: true

# Upload ASC Routing App Coverage File (.geojson, single MultiPolygon).
# Required when App Store Connect shows "Routing App Coverage File" on the
# version page (triggered if a build declared MKDirections modes, or ASC asks
# for geographic coverage). KubePilot ships a worldwide MultiPolygon.
#
# Requires: ASC_KEY_ID, ASC_ISSUER_ID, ASC_KEY_FILEPATH
# Optional: ASC_APP_ID (default 6791872512), ROUTING_COVERAGE_PATH

require "spaceship"

APP_ID = ENV.fetch("ASC_APP_ID", "6791872512")
COVERAGE = ENV.fetch(
  "ROUTING_COVERAGE_PATH",
  File.expand_path("../fastlane/metadata/routing_app_coverage.geojson", __dir__)
)

raise "Missing coverage file at #{COVERAGE}" unless File.file?(COVERAGE)
raise "Coverage file must be .geojson" unless File.extname(COVERAGE).casecmp(".geojson").zero?

Spaceship::ConnectAPI.token = Spaceship::ConnectAPI::Token.create(
  key_id: ENV.fetch("ASC_KEY_ID"),
  issuer_id: ENV.fetch("ASC_ISSUER_ID"),
  filepath: ENV.fetch("ASC_KEY_FILEPATH")
)

app = Spaceship::ConnectAPI::App.get(app_id: APP_ID)
raise "App #{APP_ID} not found" unless app

version = app.get_edit_app_store_version(platform: Spaceship::ConnectAPI::Platform::IOS)
raise "No editable iOS App Store version for #{APP_ID}" unless version

puts "Uploading routing coverage #{File.basename(COVERAGE)} → version #{version.version_string} (#{version.id})…"

if defined?(Spaceship::ConnectAPI::RoutingAppCoverage)
  # Replace any existing coverage on this version.
  begin
    existing = version.routing_app_coverage if version.respond_to?(:routing_app_coverage)
    if existing.respond_to?(:delete!)
      existing.delete!
      puts "Removed previous routing coverage"
    end
  rescue StandardError => e
    warn "Could not clear prior coverage: #{e.message}"
  end

  Spaceship::ConnectAPI::RoutingAppCoverage.create(
    app_store_version_id: version.id,
    path: COVERAGE
  )
  puts "Uploaded routing app coverage (worldwide MultiPolygon)"
else
  raise "This fastlane build has no RoutingAppCoverage API — upgrade fastlane"
end

puts "Done."
