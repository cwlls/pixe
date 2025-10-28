import pathlib
import os
import shutil
import tempfile

import pytest

# from PIL import Image
# from PIL import ImageColor
# import piexif

DATA_DIR = pathlib.Path(os.path.dirname(__file__)).joinpath("data")


@pytest.fixture
def src_path(sandbox):
    yield sandbox.joinpath("src")


@pytest.fixture
def dst_path(sandbox):
    path = sandbox.joinpath("dst")
    path.mkdir()
    yield path


@pytest.fixture
def test_files():
    yield TEST_FILES


@pytest.fixture(autouse=True)
def sandbox():
    with tempfile.TemporaryDirectory() as tmpdir:
        shutil.copytree(DATA_DIR, f"{tmpdir}/src")
        yield pathlib.Path(tmpdir)


# def pytest_configure():
#     print("Setting up test files")
#     src_dir = DATA_DIR.joinpath("src")
#     light_dir = src_dir.joinpath("light")
#     light_dir.mkdir(parents=True)
#     dark_dir = src_dir.joinpath("dark")
#     dark_dir.mkdir()

#     # Create color files
#     date_idx = 0
#     for color in LIGHT_COLORS:
#         create_img(color, DATES[date_idx], light_dir)
#         date_idx += 1

#     for color in DARK_COLORS:
#         create_img(color, DATES[date_idx], dark_dir)
#         date_idx += 1

#     for color in COLORS:
#         create_img(color, DATES[date_idx], src_dir)
#         date_idx += 1

#     # create file with no exif date
#     create_img("chocolate", None, src_dir)


# def pytest_unconfigure():
#     print("Tearing down test files")
#     shutil.rmtree(DATA_DIR)


# a few helper variables and functions


# def create_img(color: str, date: str, dest: pathlib.Path):
#     if date:
#         exif_ifd = {
#             piexif.ExifIFD.DateTimeOriginal: date,
#         }
#     else:
#         exif_ifd = {}

#     exif_bytes = piexif.dump({"Exif": exif_ifd})
#     im = Image.new("RGB", (640, 480), ImageColor.getrgb(ImageColor.colormap.get(color)))
#     im.save(dest.joinpath(f"{color}.jpg"), "JPEG", exif=exif_bytes)


# CAMERAS = [
#     {"make": "Motorola", "model": "MotoG3"},
#     {"make": "PENTAX Corporation", "model": "PENTAX K100D"},
#     {"make": "Apple", "model": "iPhone SE (2nd generation)"},
# ]

# DATES = [
#     "2021:03:29 00:40:52",
#     "2019:04:05 09:42:01",
#     "2022:11:09 00:28:55",
#     "2019:11:13 21:43:49",
#     "2021:05:23 04:30:09",
#     "2018:03:19 10:01:39",
#     "2018:10:29 17:37:58",
#     "2021:06:14 05:31:14",
#     "2022:11:29 18:12:01",
#     "2022:04:11 19:00:08",
#     "2022:05:02 18:29:01",
#     "2022:01:15 13:08:41",
#     "2020:12:09 01:55:01",
#     "2022:02:26 00:18:21",
#     "2020:03:21 03:13:12",
#     "2021:12:22 15:38:25",
#     "2022:08:03 04:54:30",
#     "2019:03:05 18:19:02",
#     "2022:07:13 16:32:28",
#     "2019:07:05 01:22:14",
#     "2020:05:12 12:04:00",
# ]

# COLORS = [
#     "red",
#     "orange",
#     "yellow",
#     "green",
#     "blue",
#     "indigo",
#     "violet",
# ]

# DARK_COLORS = [
#     "darkred",
#     "darkorange",
#     "darkgoldenrod",
#     "darkgreen",
#     "darkblue",
#     "darkturquoise",
#     "darkviolet",
# ]

# LIGHT_COLORS = [
#     "lightpink",
#     "lightcoral",
#     "lightyellow",
#     "lightgreen",
#     "lightblue",
#     "lightseagreen",
#     "lightsteelblue",
# ]

TEST_FILES = {
    "yellow": {
        "year": "2022",
        "month": "08-Aug",
        "dstamp": "20220803",
        "tstamp": "045430",
        "checksum": "14f27422440b275b29c3445685088705542b5785",
    },
    "lightsteelblue": {
        "year": "2018",
        "month": "10-Oct",
        "dstamp": "20181029",
        "tstamp": "173758",
        "checksum": "2322bd54435c6e5587922f581c3584d8f306df1b",
    },
    "lightblue": {
        "year": "2021",
        "month": "05-May",
        "dstamp": "20210523",
        "tstamp": "043009",
        "checksum": "b3808ba52ad8f43f84f62cf161432a379ad28b24",
    },
    "lightpink": {
        "year": "2021",
        "month": "03-Mar",
        "dstamp": "20210329",
        "tstamp": "004052",
        "checksum": "eb604bd834eb9232f05391274a11bfeeb4261acf",
    },
    "orange": {
        "year": "2021",
        "month": "12-Dec",
        "dstamp": "20211222",
        "tstamp": "153825",
        "checksum": "d05cae67991384d221e95ae8b30994ce186695ed",
    },
    "lightseagreen": {
        "year": "2018",
        "month": "03-Mar",
        "dstamp": "20180319",
        "tstamp": "100139",
        "checksum": "b78473e5c10d8fd945dd1eee9da7a82320d464d1",
    },
    "lightcoral": {
        "year": "2019",
        "month": "04-Apr",
        "dstamp": "20190405",
        "tstamp": "094201",
        "checksum": "b1ca56fce9618551bd2025d18d363d51a4330768",
    },
    "darkturquoise": {
        "year": "2020",
        "month": "12-Dec",
        "dstamp": "20201209",
        "tstamp": "015501",
        "checksum": "a810b8552a4acf4e13164a74aab3016e583cc93e",
    },
    "darkviolet": {
        "year": "2022",
        "month": "02-Feb",
        "dstamp": "20220226",
        "tstamp": "001821",
        "checksum": "476bf667385499407e1405f5909f88875dab1873",
    },
    "darkorange": {
        "year": "2022",
        "month": "11-Nov",
        "dstamp": "20221129",
        "tstamp": "181201",
        "checksum": "043fc29677f453d324787080c33464840235b581",
    },
    "darkgoldenrod": {
        "year": "2022",
        "month": "04-Apr",
        "dstamp": "20220411",
        "tstamp": "190008",
        "checksum": "14c016e86d7d31b87b4b95e4d0ede3c17e15df34",
    },
    "indigo": {
        "year": "2019",
        "month": "07-Jul",
        "dstamp": "20190705",
        "tstamp": "012214",
        "checksum": "786e7aefcbe4623422d15f805d6353f64b232801",
    },
    "blue": {
        "year": "2022",
        "month": "07-Jul",
        "dstamp": "20220713",
        "tstamp": "163228",
        "checksum": "6318c7dffbc4540f55eca0210f9498b067f28964",
    },
    "darkblue": {
        "year": "2022",
        "month": "01-Jan",
        "dstamp": "20220115",
        "tstamp": "130841",
        "checksum": "d9821e596764fb0657b914e0e657d198602d3bef",
    },
    "darkred": {
        "year": "2021",
        "month": "06-Jun",
        "dstamp": "20210614",
        "tstamp": "053114",
        "checksum": "0e6a2ef81906c98e4735fc9aad4bc6adaae42669",
    },
    "darkgreen": {
        "year": "2022",
        "month": "05-May",
        "dstamp": "20220502",
        "tstamp": "182901",
        "checksum": "6245c093f3e3d43d190688bed0cfd78127c8f799",
    },
    "lightyellow": {
        "year": "2022",
        "month": "11-Nov",
        "dstamp": "20221109",
        "tstamp": "002855",
        "checksum": "85817cbca21624e78da7554d02cc81b184d63a39",
    },
    "violet": {
        "year": "2020",
        "month": "05-May",
        "dstamp": "20200512",
        "tstamp": "120400",
        "checksum": "cf46355fc55b284e2889113c6d98bf01e16978c1",
    },
    "red": {
        "year": "2020",
        "month": "03-Mar",
        "dstamp": "20200321",
        "tstamp": "031312",
        "checksum": "1cdef99be68dbdea159ec6fa8469b41ca13e9e6f",
    },
    "chocolate": {
        "year": "1902",
        "month": "02-Feb",
        "dstamp": "19020220",
        "tstamp": "000000",
        "checksum": "2a00d2b48e39f63cf834d4f7c50b2c1aa3b43a9c",
    },
    "green": {
        "year": "2019",
        "month": "03-Mar",
        "dstamp": "20190305",
        "tstamp": "181902",
        "checksum": "54d7cb91b979c2287e09044caa34a267966f980c",
    },
    "lightgreen": {
        "year": "2019",
        "month": "11-Nov",
        "dstamp": "20191113",
        "tstamp": "214349",
        "checksum": "bbde896f638882158d104100d81e5d1939742c20",
    },
}
