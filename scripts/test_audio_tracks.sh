#!/bin/bash
# Test script to verify audio track detection for MKV files

echo "Audio Track Detection Test"
echo "=========================="
echo ""

if [ -z "$1" ]; then
    echo "Usage: $0 <path-to-mkv-file>"
    echo ""
    echo "This script will show all audio tracks in the MKV file"
    echo "and simulate which track would be selected."
    exit 1
fi

FILE="$1"

if [ ! -f "$FILE" ]; then
    echo "Error: File not found: $FILE"
    exit 1
fi

echo "Analyzing: $FILE"
echo ""

echo "All audio tracks:"
echo "-----------------"
ffprobe -v error -select_streams a -show_entries stream=index,codec_name:stream_tags=language -of csv=p=0 "$FILE"
echo ""

echo "First audio track (old behavior):"
echo "-----------------------------------"
ffprobe -v error -select_streams a:0 -show_entries stream=codec_name -of default=noprint_wrappers=1:nokey=1 "$FILE"
echo ""

echo "English audio tracks (new behavior preference):"
echo "------------------------------------------------"
ffprobe -v error -select_streams a -show_entries stream=index,codec_name:stream_tags=language -of csv=p=0 "$FILE" | grep -E ",eng$|,en$" || echo "No English audio tracks found"
echo ""

echo "Video codec info:"
echo "-----------------"
ffprobe -v error -select_streams v:0 -show_entries stream=codec_name -of default=noprint_wrappers=1:nokey=1 "$FILE"

