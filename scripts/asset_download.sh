#!/bin/bash

# Create necessary directories for static assets
mkdir -p internal/server/static/static/css
mkdir -p internal/server/static/static/js
mkdir -p internal/server/static/static/fonts

echo "Downloading Tailwind CSS..."
# Download Tailwind CSS (standalone version)
curl -sL "https://cdn.tailwindcss.com" -o "internal/server/static/static/js/tailwindcss.js"

echo "Downloading Lucide Icons..."
# Download Lucide Icons (UMD version for browser)
curl -sL "https://unpkg.com/lucide@latest/dist/umd/lucide.min.js" -o "internal/server/static/static/js/lucide.min.js"

echo "Downloading HLS.js..."
# Download HLS.js for video streaming
curl -sL "https://cdn.jsdelivr.net/npm/hls.js@latest/dist/hls.min.js" -o "internal/server/static/static/js/hls.min.js"

echo "Downloading Inter font..."
# Download Inter font from Google Fonts
curl -sL "https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" -A "Mozilla/5.0" -o "internal/server/static/static/css/inter.css"

# Extract font URLs from the CSS and download them
echo "Downloading Inter font files..."
grep -o "https://fonts.gstatic.com/s/inter/[^)]*" internal/server/static/static/css/inter.css | while read -r url; do
  filename=$(basename "$url")
  curl -sL "$url" -o "internal/server/static/static/fonts/$filename"
done

# Update font CSS to use local files
echo "Updating Inter font CSS paths..."
sed -i.bak 's|https://fonts.gstatic.com/s/inter/v[0-9]*/|/static/fonts/|g' internal/server/static/static/css/inter.css
rm -f internal/server/static/static/css/inter.css.bak

echo "All assets downloaded successfully!"
