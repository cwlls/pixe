import logging
import datetime
import hashlib
import io
import pathlib

import PIL.Image
import piexif

import filetypes
import base

FACTORY = filetypes.factory
LOGGER = logging.getLogger(__name__)


class ImageFile(base.PixeFile):
    """
    Image files
    """
    EXTENSIONS = ["jpg", "jpeg"]
    ALLOWED_TAGS = ["copyright", "owner"]

    def __init__(self, path: pathlib.Path):
        super().__init__(path)

    @property
    def checksum(self, block_size: int = 8192) -> str:
        """
        Create a sha1 checksum of just the image data (no meta/exif).

        :param block_size: the block size to use when chunking up the image data
        :return: a calculated hex digest
        """
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

        return hasher.hexdigest()

    @property
    def creation_date(self) -> datetime.datetime:
        """
        Extract the file creation date from EXIF information.

        :return: a datetime object representing the creation date of the image
        """
        # TODO: use piexif: exif_dict['Exif'][piexif.ExifIFD.DateTimeOriginal]
        with PIL.Image.open(self.path, "r") as im:
            try:
                # attempt to extract the creation date from EXIF tag 36867
                exif = im._getexif()
                cdate = datetime.datetime.strptime(exif[36867], "%Y:%m:%d %H:%M:%S")

            # the requested tag doesn't exist, use the ERROR_DATE global to signify such
            except KeyError:
                cdate = self.DEFAULT_DATE

            return cdate

    @property
    def metadata(self):
        return piexif.load(str(self.path))

    @classmethod
    def add_metadata(cls, file: pathlib.Path, **kwargs):
        assert file.suffix.lstrip('.').lower() in cls.EXTENSIONS
        for key in kwargs.keys():
            assert key in cls.ALLOWED_TAGS

        exif_data = piexif.load(str(file))

        if "owner" in kwargs and kwargs.get("owner") != '':
            exif_data["Exif"][0xa430] = kwargs.get("owner").encode("ascii")
        if "copyright" in kwargs and kwargs.get("copyright") != '':
            exif_data["0th"][piexif.ImageIFD.Copyright] = kwargs.get("copyright").encode("ascii")

        try:
            exif_bytes = piexif.dump(exif_data)
        except ValueError as e:
            LOGGER.info(f"{e}: {str(file)}")
            del exif_data["Exif"][41729]
            exif_bytes = piexif.dump(exif_data)
        finally:
            piexif.insert(exif_bytes, str(file))


# add ImageFile extensions and creator method to the Factory
for ext in ImageFile.EXTENSIONS:
    FACTORY.register_filetype(ext, ImageFile)
