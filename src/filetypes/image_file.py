import logging
import datetime
import hashlib
import io
import pathlib

import PIL.Image
import exiftool
import piexif
from pillow_heif import HeifImagePlugin

import filetypes
from . import base

FACTORY = filetypes.factory
LOGGER = logging.getLogger(__name__)


class ImageFile(base.PixeFile):
    """
    Image files
    """

    EXTENSIONS = ["jpg", "jpeg", "heic", "heif"]
    # ALLOWED_TAGS = ["copyright", "owner"]
    ALLOWED_TAGS = []

    def __init__(self, path: pathlib.Path):
        super().__init__(path)

    @property
    def checksum(self, block_size: int = 8192) -> str:
        hasher = hashlib.sha1()
        img_io = io.BytesIO()

        # open the image file and save the image data portion as an io.BytesIO object
        with PIL.Image.open(self.path) as im:
            im.save(img_io, im.format)

        # reset the cursor
        img_io.seek(0)

        # chunk_size at a time, update our hash until complete
        while chunk := img_io.read(block_size):
            hasher.update(chunk)

        chksum = hasher.hexdigest()
        LOGGER.info(f"CHECKSUM: {chksum}")

        return chksum

    @property
    def creation_date(self) -> datetime.datetime:
        with exiftool.ExifToolHelper() as et:
            exif = et.get_metadata(self.path)[0]
            try:
                cdate = datetime.datetime.strptime(
                    exif["EXIF:DateTimeOriginal"], "%Y:%m:%d %H:%M:%S"
                )
                LOGGER.debug(f"{self.path}: {cdate}")
            except exiftool.exceptions.ExifToolTagNameError as e:
                LOGGER.error(f"{e}")
                cdate = self.DEFAULT_DATE

        return cdate

    ## TODO: this needs to be rewritten to accomodate filetypes other than jpg.
    ##  also need to fix the ALLOWED_TAGS constant above when done with rewrite.
    # @property
    # def metadata(self):
    #     return piexif.load(str(self.path))

    # @classmethod
    # def add_metadata(cls, file: pathlib.Path, **kwargs):
    #     assert file.suffix.lstrip(".").lower() in cls.EXTENSIONS
    #     for key in kwargs.keys():
    #         assert key in cls.ALLOWED_TAGS

    #     exif_data = piexif.load(str(file))

    #     if "owner" in kwargs and kwargs.get("owner") != "":
    #         exif_data["Exif"][0xA430] = kwargs.get("owner").encode("ascii")
    #     if "copyright" in kwargs and kwargs.get("copyright") != "":
    #         exif_data["0th"][piexif.ImageIFD.Copyright] = kwargs.get(
    #             "copyright"
    #         ).encode("ascii")

    #     try:
    #         exif_bytes = piexif.dump(exif_data)
    #     except ValueError as e:
    #         LOGGER.error(f"{e}: {str(file)}")
    #         del exif_data["Exif"][41729]
    #         exif_bytes = piexif.dump(exif_data)
    #     finally:
    #         piexif.insert(exif_bytes, str(file))


# add ImageFile extensions and creator method to the Factory
for ext in ImageFile.EXTENSIONS:
    FACTORY.register_filetype(ext, ImageFile)
