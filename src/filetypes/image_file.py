import logging
import datetime
import hashlib
import io
import pathlib

import PIL.Image
import PIL.ExifTags
from pillow_heif import HeifImagePlugin
import exiftool

import filetypes
from . import base

FACTORY = filetypes.factory
LOGGER = logging.getLogger(__name__)


class ImageFile(base.PixeFile):
    """
    Image files
    """

    EXTENSIONS = ["jpg", "jpeg", "heic", "heif"]
    ALLOWED_TAGS = ["owner"]

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
        with PIL.Image.open(self.path) as im:
            exif = im.getexif()
            exif_ifd = exif.get_ifd(PIL.ExifTags.IFD.Exif)
        try:
            cdate = datetime.datetime.strptime(exif_ifd[PIL.ExifTags.Base.DateTimeOriginal], "%Y:%m:%d %H:%M:%S")
        except KeyError as e:
            LOGGER.info(f"{e}")
            cdate = self.DEFAULT_DATE

        return cdate

    @property
    def metadata(self):
        with PIL.Image.open(self.path) as im:
            return im.getexif().get_ifd(PIL.ExifTags.IFD.Exif)

    @classmethod
    def add_metadata(cls, file: pathlib.Path, **kwargs):
        for tag in kwargs.keys():
            assert tag in cls.ALLOWED_TAGS, f"'{tag}' is not in the list of allowed tags!"

        new_tags = {}
        if owner := kwargs.get("owner"):
            new_tags["OwnerName"] = owner
        # TODO: figure out a way to grab the date of the file and use that for
        #  the copyright year
        # if copyright := kwargs.get("copyright"):
        #     new_tags["Copyright"] = copyright
        if new_tags:
            with exiftool.ExifToolHelper() as et:
                et.set_tags(file, tags=new_tags, params=["-P", "-overwrite_original"])


# add ImageFile extensions and creator method to the Factory
for ext in ImageFile.EXTENSIONS:
    FACTORY.register_filetype(ext, ImageFile)
