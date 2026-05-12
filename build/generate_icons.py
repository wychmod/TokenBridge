from pathlib import Path
from PIL import Image, ImageDraw, ImageFilter

ROOT = Path(__file__).resolve().parents[1]
ASSETS = ROOT / "build" / "assets"
WEB_PUBLIC = ROOT / "web" / "admin" / "public"
ASSETS.mkdir(parents=True, exist_ok=True)
WEB_PUBLIC.mkdir(parents=True, exist_ok=True)

size = 512
img = Image.new("RGBA", (size, size), (7, 17, 31, 0))
draw = ImageDraw.Draw(img)

for i, alpha in enumerate([36, 24, 16]):
    inset = 28 + i * 26
    draw.rounded_rectangle(
        (inset, inset, size - inset, size - inset),
        radius=120 - i * 18,
        fill=(12 + i * 10, 22 + i * 10, 38 + i * 14, alpha),
        outline=(88, 132, 255, 34),
        width=2,
    )

core = Image.new("RGBA", (size, size), (0, 0, 0, 0))
core_draw = ImageDraw.Draw(core)
core_draw.rounded_rectangle((84, 84, 428, 428), radius=120, fill=(10, 24, 42, 255))
core = core.filter(ImageFilter.GaussianBlur(0.5))
img.alpha_composite(core)

grad = Image.new("RGBA", (size, size), (0, 0, 0, 0))
grad_draw = ImageDraw.Draw(grad)
for y in range(size):
    ratio = y / (size - 1)
    r = int(122 + (56 - 122) * ratio)
    g = int(92 + (189 - 92) * ratio)
    b = int(255 + (248 - 255) * ratio)
    grad_draw.line((0, y, size, y), fill=(r, g, b, 255))
mask = Image.new("L", (size, size), 0)
mask_draw = ImageDraw.Draw(mask)
mask_draw.rounded_rectangle((104, 104, 408, 408), radius=110, fill=255)
grad.putalpha(mask)
img.alpha_composite(grad)

highlight = Image.new("RGBA", (size, size), (0, 0, 0, 0))
highlight_draw = ImageDraw.Draw(highlight)
highlight_draw.ellipse((118, 88, 350, 252), fill=(255, 255, 255, 54))
highlight = highlight.filter(ImageFilter.GaussianBlur(18))
img.alpha_composite(highlight)

symbol = Image.new("RGBA", (size, size), (0, 0, 0, 0))
symbol_draw = ImageDraw.Draw(symbol)
symbol_draw.rounded_rectangle((168, 128, 238, 384), radius=28, fill=(255, 255, 255, 245))
symbol_draw.rounded_rectangle((168, 314, 356, 384), radius=28, fill=(255, 255, 255, 245))
symbol_draw.rounded_rectangle((278, 128, 348, 272), radius=28, fill=(255, 255, 255, 245))
symbol_draw.rounded_rectangle((278, 202, 402, 272), radius=28, fill=(255, 255, 255, 245))
symbol = symbol.filter(ImageFilter.GaussianBlur(0.2))
img.alpha_composite(symbol)

shadow = Image.new("RGBA", (size, size), (0, 0, 0, 0))
shadow_draw = ImageDraw.Draw(shadow)
shadow_draw.rounded_rectangle((96, 96, 416, 416), radius=118, outline=(255, 255, 255, 44), width=4)
shadow = shadow.filter(ImageFilter.GaussianBlur(0.8))
img.alpha_composite(shadow)

png_path = ASSETS / "app-icon.png"
img.save(png_path)

favicon_path = WEB_PUBLIC / "favicon.png"
img.resize((64, 64), Image.LANCZOS).save(favicon_path)

ico_path = ASSETS / "app-icon.ico"
img.save(ico_path, sizes=[(256, 256), (128, 128), (64, 64), (48, 48), (32, 32), (16, 16)])

tray_path = ASSETS / "tray-icon.ico"
img.resize((64, 64), Image.LANCZOS).save(tray_path, sizes=[(64, 64), (32, 32), (16, 16)])

print("generated", png_path)
print("generated", favicon_path)
print("generated", ico_path)
print("generated", tray_path)
