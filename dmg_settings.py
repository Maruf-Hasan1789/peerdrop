import os.path

# -- Appearance Settings --
filename = 'PeerDrop-Installer.dmg'
volume_name = 'PeerDrop'
# You can add a path to a .icns file here if you have one
# icon = 'build/appicon.icns'

# -- Contents --
# This maps the app and the Applications shortcut
# The coordinates are (x, y)
app_path = os.path.join('build/bin', 'PeerDrop.app')
contents = {
    app_path: (150, 150),
    '/Applications': (450, 150),
}

# -- Window Settings --
window_rect = ((600, 600), (600, 400))
default_view = 'icon-view'
show_status_bar = False
show_tab_view = False
show_toolbar = False
show_sidebar = False
icon_size = 100