#!/bin/bash

OUTPUT_DIR="./github_secrets"
mkdir -p $OUTPUT_DIR

encode_file() {
  local file=$1
  local output_file=$2
  if [ -f "$file" ]; then
    if [[ "$OSTYPE" == "darwin"* ]]; then
      base64 -i "$file" > "$output_file"
    else
      base64 "$file" > "$output_file"
    fi
    echo "Encoded $file to $output_file"
  else
    echo "Error: $file does not exist"
    exit 1
  fi
}

echo "This script will help you prepare certificate files for GitHub Actions secrets."
echo "You need the following files:"
echo "1. Developer ID Application certificate (.p12)"
echo "2. Developer ID Installer certificate (.p12)"
echo ""

# Get paths to certificate files
read -p "Path to Developer ID Application certificate (.p12): " APP_CERT_PATH
read -p "Path to Developer ID Installer certificate (.p12): " INSTALLER_CERT_PATH

# Encode certificate files
encode_file "$APP_CERT_PATH" "$OUTPUT_DIR/application_cert.txt"
encode_file "$INSTALLER_CERT_PATH" "$OUTPUT_DIR/installer_cert.txt"