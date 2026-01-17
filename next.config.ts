import type { NextConfig } from "next";

const backendPort = process.env.BACKEND_PORT || "3001";

const nextConfig: NextConfig = {
  // Static Export for Hono backend serving
  output: "export",
  // Optimize images for static export
  images: {
    unoptimized: true,
  },
  // Trailing slash for cleaner static paths
  trailingSlash: true,
  async rewrites() {
    if (process.env.NODE_ENV === "development") {
      return [
        {
          source: "/api/:path*",
          destination: `http://localhost:${backendPort}/api/:path*`,
        },
      ];
    }
    return [];
  },
};

export default nextConfig;
