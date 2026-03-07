#!/bin/bash
set -e  # Exit immediately on any error

# Configuration
APP_NAME="peerdrop"          # Must be lowercase for Debian policy
APP_BINARY="peerdropApp"     # Actual Wails binary name (from wails.json outputfilename)
VERSION="0.0.1~beta2"
MAINTAINER="Maruf Hasan <marufhasan212@gmail.com>"
UPLOADERS=""
HOMEPAGE="https://github.com/Maruf-Hasan1789/peerdrop"
DESCRIPTION="A fast and simple peer-to-peer file sharing application built with Wails."
DEB_FILE="${APP_NAME}_${VERSION}_amd64.deb"

# 1. Build the Wails binary
echo "Building Wails binary..."
wails build -platform linux/amd64

# 2. Create folder structure
echo "Preparing folder structure..."
rm -rf build/debian_dist
mkdir -p build/debian_dist/DEBIAN
mkdir -p build/debian_dist/usr/bin
mkdir -p build/debian_dist/usr/share/applications
mkdir -p build/debian_dist/usr/share/pixmaps

# 3. Copy binary
cp "build/bin/$APP_BINARY" "build/debian_dist/usr/bin/$APP_NAME"

# 4. Locate and copy icon
ICON_SRC=""
for candidate in \
    "build/appicon.png" \
    "build/darwin/appicon.png" \
    "frontend/src/assets/images/appicon.png" \
    "appicon.png"; do
    if [ -f "$candidate" ]; then
        ICON_SRC="$candidate"
        break
    fi
done

if [ -n "$ICON_SRC" ]; then
    cp "$ICON_SRC" "build/debian_dist/usr/share/pixmaps/$APP_NAME.png"
    echo "Icon found at: $ICON_SRC"
else
    echo "Warning: No icon found. The app will install without an icon."
fi

# 5. Calculate installed size (required by Debian)
INSTALLED_SIZE=$(du -sk build/debian_dist/usr | cut -f1)

# 6. Create control file
echo "Creating Debian control file..."

cat <<EOF > build/debian_dist/DEBIAN/control
Package: $APP_NAME
Version: $VERSION
Section: utils
Priority: optional
Architecture: amd64
Installed-Size: $INSTALLED_SIZE
Maintainer: $MAINTAINER
Homepage: $HOMEPAGE
Depends: libgtk-3-0, libwebkit2gtk-4.1-0 | libwebkit2gtk-4.0-37, libgdk-pixbuf-2.0-0 | libgdk-pixbuf2.0-0
Description: $DESCRIPTION
EOF

# Add Uploaders field if provided
if [ -n "$UPLOADERS" ]; then
    echo "Uploaders: $UPLOADERS" >> build/debian_dist/DEBIAN/control
fi

chmod 644 build/debian_dist/DEBIAN/control

# 7. Create desktop entry
cat <<EOF > build/debian_dist/usr/share/applications/$APP_NAME.desktop
[Desktop Entry]
Name=PeerDrop
Comment=Fast peer-to-peer file sharing
Exec=/usr/bin/$APP_NAME
Icon=$APP_NAME
Type=Application
Terminal=false
Categories=Network;Utility;
EOF

chmod 644 build/debian_dist/usr/share/applications/$APP_NAME.desktop

# 8. Set binary permissions
chmod 755 build/debian_dist/usr/bin/$APP_NAME

# 9. Fix directory permissions (required by dpkg)
find build/debian_dist -type d -exec chmod 755 {} \;

# 10. Build the .deb package
echo "Packaging .deb file..."
dpkg-deb --root-owner-group --build build/debian_dist "$DEB_FILE"

echo ""
echo "================================================"
echo "Success! Package created: $DEB_FILE"
echo "================================================"
echo ""
echo "Install with:"
echo "  sudo apt install ./$DEB_FILE"
echo ""
echo "Or:"
echo "  sudo dpkg -i $DEB_FILE && sudo apt-get install -f"
echo ""
echo "Uninstall with:"
echo "  sudo apt remove $APP_NAME"