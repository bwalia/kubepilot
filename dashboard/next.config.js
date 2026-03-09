/** @type {import('next').NextConfig} */
const nextConfig = {
  output: "export", // Static export for Go server to serve from ./dashboard/out
  trailingSlash: true,
  // Proxy API calls to the Go backend during development.
  async rewrites() {
    return process.env.NODE_ENV === "development"
      ? [{ source: "/api/:path*", destination: "http://localhost:8383/api/:path*" }]
      : [];
  },
};

module.exports = nextConfig;
