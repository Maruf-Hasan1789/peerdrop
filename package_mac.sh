#!/bin/bash

APP_NAME="PeerDrop"
APP_PATH="build/bin/$APP_NAME.app"
DMG_TEMP="dmg_temp"
DMG_NAME="$APP_NAME-Installer.dmg"

echo "Cleaning up..."
rm -rf "$DMG_TEMP" "$DMG_NAME"
mkdir -p "$DMG_TEMP"

echo "Re-signing app..."
codesign --force --deep -s - "$APP_PATH"

echo "Preparing DMG contents..."
cp -R "$APP_PATH" "$DMG_TEMP/"
# This creates the 'Applications' shortcut inside the DMG
ln -s /Applications "$DMG_TEMP/Applications"

echo "Creating DMG..."
hdiutil create -volname "$APP_NAME" -srcfolder "$DMG_TEMP" -ov -format UDZO "$DMG_NAME"

echo "Cleaning up temp files..."
rm -rf "$DMG_TEMP"

echo "Done! $DMG_NAME is ready for distribution."