# Studio One Song Converter

A cross-platform GUI app to downgrade Studio One `.song` files between versions 8, 7 and 6, based on my script here:
https://github.com/djimc/Fender-Studio-Pro-Studio-One-.song-converter

## Features
- Drag & drop or browse for `.song` files
- Auto-detects project version and shows only valid conversion options
- Applies all required patches (FormatVersion + Pro EQ UUID fix)
- Studio One-inspired dark theme


I am not planning to fix any issues because the app was created just to test how far AI has gone. I will probably update the app (and the script) when v9 is released if I decide to upgrade to Fender Studio 9 when it arrives.

I have not included conversion (downgrades) to pre-6 versions because they don't support Dynamic EQ and I see no reason to go back to any verion below 6.0. This is something I creaded for myself and I'm sharing with the world. I have tested on Windows 11 and Debian Trixie. I have NOT tested the Mac releases.

Anyone interested to pick up, please share with the world ;)
Cheers!
---


## Usage

1. Launch the app
2. Drag a `.song` file onto the drop zone, or click **Browse**
3. The app detects the project version and shows available conversions
4. Click a conversion button — the patched file is saved next to the original

| Detected Version   | Available Conversions              |
|--------------------|------------------------------------|
| Studio Pro v8 (9)  | → Studio One 7 (v8), → SO6 (v7)   |
| Studio One v7 (8)  | → Studio One 6 (v7)                |
