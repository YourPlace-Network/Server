#!/bin/bash

APP_CERTIFICATE="Developer ID Application: Austin Lawrence (2NNLSL5QT4)"
INSTALLER_CERTIFICATE="Developer ID Installer: Austin Lawrence (2NNLSL5QT4)"

# Development Mode
DEV_MODE=0
DEV_ARG=""
if [ "$1" == "dev" ]; then
    DEV_MODE=1
    DEV_ARG="<string>-d=true</string>"
fi

# Extract version number from main.go
VERSION=$(grep 'version.*=.*".*"' main.go | sed -E 's/.*version.*=.*"(.*)".*/\1/')
if [ -z "$VERSION" ]; then
    echo "Error: Could not extract version from main.go"
    exit 1
fi

# Create app bundle structure
mkdir -p "./target/YourPlace.app/Contents/MacOS"
mkdir -p "./target/YourPlace.app/Contents/Resources"
mkdir -p "./target/YourPlace.app/Contents/Resources/scripts"
mkdir -p "./target/Resources"
mkdir -p "./target/scripts"

# -------- LaunchDaemons & LaunchAgents -------- #
cp ./resources/osx/com.yourplace.server.plist ./target/YourPlace.app/Contents/Resources/scripts/
cp ./resources/osx/com.yourplace.uninstall.plist ./target/YourPlace.app/Contents/Resources/scripts/
cp ./resources/osx/com.yourplace.helper.plist ./target/YourPlace.app/Contents/Resources/scripts/
sed -i '' "s|\$DEV_ARG|$DEV_ARG|" ./target/YourPlace.app/Contents/Resources/scripts/com.yourplace.server.plist

# -------- Pre-Install -------- #
cp ./resources/osx/preinstall.sh ./target/scripts/preinstall
chmod 0755 ./target/scripts/preinstall

# -------- Post-Install -------- #
cp ./resources/osx/postinstall.sh ./target/scripts/postinstall
chmod 0755 ./target/scripts/postinstall

# -------- Uninstaller -------- #
cp ./resources/osx/uninstall.sh ./target/YourPlace.app/Contents/Resources/scripts/
codesign --force --sign "${APP_CERTIFICATE}" ./target/YourPlace.app/Contents/Resources/scripts/uninstall.sh
chmod 0755 ./target/YourPlace.app/Contents/Resources/scripts/uninstall.sh

# --------- Installer --------- #
cp ./target/YourPlace ./target/YourPlace.app/Contents/MacOS/
cp ./target/YourPlaceHelper ./target/YourPlace.app/Contents/MacOS/
cp ./resources/osx/AppIcon.icns ./target/YourPlace.app/Contents/Resources
cp ./src/www/image/yourplace-logo-zoomed-out.png ./target/Resources/yourplace.png
sips -Z 225 ./target/Resources/yourplace.png

# --------- Packaging --------- #
cp ./resources/osx/Info.plist ./target/YourPlace.app/Contents/
cp ./resources/osx/distribution.xml ./target/

# Create component package
pkgbuild --root "./target/YourPlace.app" \
         --install-location "/Applications/YourPlace.app" \
         --identifier "com.yourplace.server" \
         --version "${VERSION}" \
         --scripts "./target/scripts" \
         "./target/component.pkg"

# Create distribution package
productbuild --distribution "./target/distribution.xml" \
             --package-path "./target" \
             --resources "./target/Resources" \
             --version "${VERSION}" \
             "./target/YourPlace-${VERSION}.pkg"

# --------- Signing & Notarization --------- #
if [ $DEV_MODE -eq 1 ]; then
  # Cleaning app bundle of resource forks and metadata
  xattr -cr ./target/YourPlace.app
  # Sign the app package
  codesign --deep --force --sign "${APP_CERTIFICATE}" --options runtime ./target/YourPlace.app
  # Check for installer certificate
  if ! security find-identity -p codesigning | grep -q "${INSTALLER_CERTIFICATE}"; then
    echo "ERROR: Developer ID Installer certificate not found"
    echo "Please ensure your Developer ID Installer certificate is properly installed"
    exit 1
  fi
  echo "Signing and notarizing package..."
  # Sign the pkg installer
  productsign --sign "${INSTALLER_CERTIFICATE}" ./target/YourPlace-${VERSION}.pkg ./target/YourPlace-${VERSION}-signed.pkg
  if [ $? -ne 0 ]; then
    echo "ERROR: Package signing failed"
    exit 1
  fi
  # Notarize the signed package
  xcrun notarytool submit ./target/YourPlace-${VERSION}-signed.pkg --wait --keychain-profile "${NOTARYPASS}"
  if [ $? -ne 0 ]; then
    echo "ERROR: Package notarization failed"
    exit 1
  fi
  # Staple the notarization ticket
  xcrun stapler staple ./target/YourPlace-${VERSION}-signed.pkg
  if [ $? -ne 0 ]; then
    echo "ERROR: Notarization ticket stapling failed"
    exit 1
  fi
  # Clean up
  rm -f ./target/YourPlace-${VERSION}.pkg # Remove the unsigned package
  mv ./target/YourPlace-${VERSION}-signed.pkg ./target/YourPlace-${VERSION}.pkg # Rename the signed package
fi