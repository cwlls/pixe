#!//usr/bin/env python

import os
import random

from PIL import Image
from PIL import ImageColor
from faker import Faker
import piexif

IMG_DIR = './img'
FAKE = Faker()
CAMERAS = [{'make': 'Motorola', 'model': 'MotoG3'},
           {'make': 'PENTAX Corporation', 'model': 'PENTAX K100D'},
           {'make': 'Apple', 'model': 'iPhone SE (2nd generation)'}]

for k in ImageColor.colormap.keys():
    dto = FAKE.date_time_between(start_date='-2y', end_date='now')
    exif_ifd = {
        piexif.ExifIFD.DateTimeOriginal: dto.strftime('%Y:%m:%d %H:%M:%S'),
    }
    exif_bytes = piexif.dump({'Exif': exif_ifd})
    im = Image.new('RGB', (800, 600), ImageColor.getrgb(ImageColor.colormap.get(k)))
    im.save(IMG_DIR + '/' + k + '.jpg', 'JPEG', exif=exif_bytes)
