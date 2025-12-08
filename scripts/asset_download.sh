#!/bin/bash

# Create necessary directories for public assets
mkdir -p public/static/css
mkdir -p public/static/js
mkdir -p public/static/fonts

echo "Downloading Tailwind CSS..."
# Download Tailwind CSS (standalone version)
curl -sL "https://cdn.tailwindcss.com" -o "public/static/js/tailwindcss.js"

echo "Downloading Lucide Icons..."
# Download Lucide Icons (UMD version for browser)
curl -sL "https://unpkg.com/lucide@latest/dist/umd/lucide.min.js" -o "public/static/js/lucide.min.js"

echo "Downloading Inter font..."
# Download Inter font from Google Fonts
curl -sL "https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" -A "Mozilla/5.0" -o "public/static/css/inter.css"

# Extract font URLs from the CSS and download them
echo "Downloading Inter font files..."
grep -o "https://fonts.gstatic.com/s/inter/[^)]*" public/static/css/inter.css | while read -r url; do
  filename=$(basename "$url")
  curl -sL "$url" -o "public/static/fonts/$filename"
done

# Update font CSS to use local files
echo "Updating Inter font CSS paths..."
sed -i.bak 's|https://fonts.gstatic.com/s/inter/v[0-9]*/|/static/fonts/|g' public/static/css/inter.css
rm -f public/static/css/inter.css.bak

echo "All assets downloaded successfully!"
