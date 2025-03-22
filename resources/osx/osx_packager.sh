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
chmod 0755 ./target/YourPlace.app/Contents/Resources/scripts/uninstall.sh

# --------- Installer --------- #
cp ./target/YourPlace ./target/YourPlace.app/Contents/MacOS/YourPlace
cp ./target/YourPlaceHelper ./target/YourPlace.app/Contents/MacOS/YourPlaceHelper
cp ./resources/osx/AppIcon.icns ./target/YourPlace.app/Contents/Resources/AppIcon.icns
cp ./src/www/image/yourplace-logo-zoomed-out.png ./target/Resources/yourplace.png
sips -Z 225 ./target/Resources/yourplace.png

# --------- Packaging --------- #
cp ./resources/osx/Info.plist ./target/YourPlace.app/Contents/
cp ./resources/osx/distribution.xml ./target/

# --------- Signing & Notarization --------- #
if [ $DEV_MODE -eq 0 ]; then
  # Cleaning app bundle of resource forks and metadata
  xattr -cr ./target/YourPlace.app
  # Sign the app bundle and binaries
  codesign --force --timestamp --options runtime --entitlements ./resources/osx/entitlements.plist --sign "${APP_CERTIFICATE}" ./target/YourPlace.app/Contents/Resources/scripts/uninstall.sh
  codesign --force --timestamp --options runtime --entitlements ./resources/osx/entitlements.plist --sign "${APP_CERTIFICATE}" ./target/YourPlace.app/Contents/MacOS/YourPlaceHelper
  codesign --force --timestamp --options runtime --entitlements ./resources/osx/entitlements.plist --sign "${APP_CERTIFICATE}" ./target/YourPlace.app/Contents/MacOS/YourPlace
  codesign --force --timestamp --options runtime --entitlements ./resources/osx/entitlements.plist --sign "${APP_CERTIFICATE}" ./target/YourPlace.app
  # Verify the signatures to ensure it's valid
  codesign -vvv --deep --strict ./target/YourPlace.app
  if [ $? -ne 0 ]; then
    echo "ERROR: Code signing failed verification"
    exit 1
  fi
fi

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

if [ $DEV_MODE -eq 0 ]; then
  # Sign the pkg installer
  productsign --sign "${INSTALLER_CERTIFICATE}" ./target/YourPlace-${VERSION}.pkg ./target/YourPlace-${VERSION}-signed.pkg
  if [ $? -ne 0 ]; then
    echo "ERROR: Package signing failed"
    exit 1
  fi
  # Store notary credentials in the keychain
  if [ -n "${NOTARYPASS}" ]; then # Using -n to check if the variable is not empty
    xcrun notarytool store-credentials --apple-id "nops@yourplace.network" --team-id "2NNLSL5QT4" --password "${NOTARYPASS}" AustinLawrence
  else
    echo "Notary password not set in the environment, skipping as we assume that it's been set in the Github action"
  fi
  # Notarize the signed package
  xcrun notarytool submit --wait ./target/YourPlace-${VERSION}-signed.pkg --keychain-profile "AustinLawrence"
  # Verify: xcrun notarytool info -p "AustinLawrence" <UUID>
  # Staple the notarization ticket
  xcrun stapler staple ./target/YourPlace-${VERSION}-signed.pkg
  if [ $? -ne 0 ]; then
    echo "ERROR: Notarization ticket stapling failed"
    exit 1
  fi
  # Verify: spctl -a -vvv -t install YourPlace-0.1.0-signed.pkg
  # Clean up
  rm ./target/YourPlace-${VERSION}.pkg # Delete the unsigned package
  mv ./target/YourPlace-${VERSION}-signed.pkg ./target/YourPlace-${VERSION}.pkg # Rename the signed package
fi