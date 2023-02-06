import pathlib
import os
import shutil

import pytest
from PIL import Image
from PIL import ImageColor
import piexif

DATA_DIR = pathlib.Path(os.path.dirname(__file__)).joinpath('data')


@pytest.fixture
def src_path():
    return DATA_DIR.joinpath("src")


@pytest.fixture
def dst_path():
    return DATA_DIR.joinpath("dst")


def pytest_configure():
    print("Setting up test files")
    src_dir = DATA_DIR.joinpath("src")
    light_dir = src_dir.joinpath('light')
    light_dir.mkdir(parents=True)
    dark_dir = src_dir.joinpath('dark')
    dark_dir.mkdir()

    # Create color files
    date_idx = 0
    for color in LIGHT_COLORS:
        create_img(color, DATES[date_idx], light_dir)
        date_idx += 1

    for color in DARK_COLORS:
        create_img(color, DATES[date_idx], dark_dir)
        date_idx += 1

    for color in COLORS:
        create_img(color, DATES[date_idx], src_dir)
        date_idx += 1

    # create file with no exif date
    create_img('chocolate', None, src_dir)


def pytest_unconfigure():
    print("Tearing down test files")
    shutil.rmtree(DATA_DIR)


# a few helper variables and functions


def create_img(color: str, date: str, dest: pathlib.Path):
    if date:
        exif_ifd = {
            piexif.ExifIFD.DateTimeOriginal: date,
        }
    else:
        exif_ifd = {}

    exif_bytes = piexif.dump({"Exif": exif_ifd})
    im = Image.new("RGB", (640, 480), ImageColor.getrgb(ImageColor.colormap.get(color)))
    im.save(dest.joinpath(f"{color}.jpg"), "JPEG", exif=exif_bytes)


CAMERAS = [
    {"make": "Motorola", "model": "MotoG3"},
    {"make": "PENTAX Corporation", "model": "PENTAX K100D"},
    {"make": "Apple", "model": "iPhone SE (2nd generation)"},
]

DATES = [
    "2021:03:29 00:40:52",
    "2019:04:05 09:42:01",
    "2022:11:09 00:28:55",
    "2019:11:13 21:43:49",
    "2021:05:23 04:30:09",
    "2018:03:19 10:01:39",
    "2018:10:29 17:37:58",
    "2021:06:14 05:31:14",
    "2022:11:29 18:12:01",
    "2022:04:11 19:00:08",
    "2022:05:02 18:29:01",
    "2022:01:15 13:08:41",
    "2020:12:09 01:55:01",
    "2022:02:26 00:18:21",
    "2020:03:21 03:13:12",
    "2021:12:22 15:38:25",
    "2022:08:03 04:54:30",
    "2019:03:05 18:19:02",
    "2022:07:13 16:32:28",
    "2019:07:05 01:22:14",
    "2020:05:12 12:04:00",
]

COLORS = [
    "red",
    "orange",
    "yellow",
    "green",
    "blue",
    "indigo",
    "violet",
]

DARK_COLORS = [
    "darkred",
    "darkorange",
    "darkgoldenrod",
    "darkgreen",
    "darkblue",
    "darkturquoise",
    "darkviolet",
]

LIGHT_COLORS = [
    "lightpink",
    "lightcoral",
    "lightyellow",
    "lightgreen",
    "lightblue",
    "lightseagreen",
    "lightsteelblue",
]
