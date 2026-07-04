/** @type {import('next').NextConfig} */
const nextConfig = {
  // Static export: `next build` emits a static site into out/ that S3 + CloudFront serve.
  output: "export",
  images: { unoptimized: true },
  // Trailing slashes keep static routes (e.g. /login/) resolvable behind CloudFront.
  trailingSlash: true,
};

export default nextConfig;
