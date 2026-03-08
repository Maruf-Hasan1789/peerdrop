#!/usr/bin/env bash

set -e

echo "🧹 Wails Frontend Cleanup Script"
echo "--------------------------------"

PROJECT_ROOT=$(pwd)
FRONTEND_DIR="$PROJECT_ROOT/frontend"

if [ ! -d "$FRONTEND_DIR" ]; then
  echo "❌ frontend/ directory not found. Run from project root."
  exit 1
fi

echo "➡ Removing Wails generated bindings..."
rm -rf "$FRONTEND_DIR/wailsjs"

echo "➡ Removing frontend build output..."
rm -rf "$FRONTEND_DIR/dist"
rm -rf "$FRONTEND_DIR/build"

echo "➡ Removing Vite cache..."
rm -rf "$FRONTEND_DIR/node_modules/.vite"

echo "➡ Removing Wails runtime output..."
rm -rf "$PROJECT_ROOT/build"
rm -rf "$PROJECT_ROOT/bin"

echo "➡ Removing frontend lock artifacts..."
rm -rf "$FRONTEND_DIR/.eslintcache"

echo
read -p "❓ Do you want to remove node_modules and run npm install? (y/N): " choice

case "$choice" in
  y|Y )
    echo "➡ Removing node_modules..."
    rm -rf "$FRONTEND_DIR/node_modules"

    echo "📦 Installing npm dependencies..."
    (cd "$FRONTEND_DIR" && npm install)
    ;;
  * )
    echo "⏭ Skipping npm install"
    ;;
esac

echo
echo "✅ Frontend cleanup completed."
echo "👉 Run: wails dev"
