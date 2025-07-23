#!/bin/bash

mkdir -p frontend/static/css
mkdir -p frontend/static/js
mkdir -p frontend/static/webfonts
mkdir -p frontend/static/fonts

# Download Tailwind CSS
curl -sL "https://cdn.tailwindcss.com" -o "frontend/static/js/tailwindcss.js"

# Download Font Awesome 6.7.2 CSS
curl -sL https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.7.2/css/all.min.css -o frontend/static/css/all.min.css
curl -sL https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.7.2/webfonts/fa-brands-400.woff2 -o frontend/static/webfonts/fa-brands-400.woff2
curl -sL https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.7.2/webfonts/fa-regular-400.woff2 -o frontend/static/webfonts/fa-regular-400.woff2
curl -sL https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.7.2/webfonts/fa-solid-900.woff2 -o frontend/static/webfonts/fa-solid-900.woff2
curl -sL https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.7.2/webfonts/fa-v4compatibility.woff2 -o frontend/static/webfonts/fa-v4compatibility.woff2

# Update the CSS to use the local webfonts path
sed 's|https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.7.2/webfonts/|/static/webfonts/|g' frontend/static/css/all.min.css > frontend/static/css/all.min.css.tmp && mv frontend/static/css/all.min.css.tmp frontend/static/css/all.min.css

# Download Inter font from Google Fonts
curl -sL "https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" -A "Mozilla/5.0" -o frontend/static/css/inter.css

# Download font files referenced in the CSS
grep -o "https://fonts.gstatic.com/s/inter/[^)]*" frontend/static/css/inter.css | while read -r url; do
  filename=$(basename "$url")
  curl -sL "$url" -o "frontend/static/fonts/$filename"
done

# Update font CSS to use local files
sed -i.bak 's|https://fonts.gstatic.com/s/inter/v[0-9]*/|/static/fonts/|g' frontend/static/css/inter.css
rm frontend/static/css/inter.css.bak

# Download Plyr JS and CSS
curl -sL https://cdn.plyr.io/3.7.8/plyr.css -o frontend/static/css/plyr.css
curl -sL https://cdn.plyr.io/3.7.8/plyr.js -o frontend/static/js/plyr.js

# Download Plyr SVG icon referenced in its CSS
curl -sL "https://cdn.plyr.io/static/plyr/3.7.8/plyr.svg" -o "frontend/static/css/plyr.svg"

# Update Plyr CSS to use the local SVG icon
sed -i.bak 's|https://cdn.plyr.io/static/plyr/3.7.8/plyr.svg|plyr.svg|g' frontend/static/css/plyr.css
rm frontend/static/css/plyr.css.bak

echo "All assets downloaded!"
