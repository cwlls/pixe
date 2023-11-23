import logging
import datetime
import hashlib
import io
import fnmatch

import PIL.Image
import piexif

import filetypes
APP = filetypes.APP

LOGGER = logging.getLogger(__name__)


class ImageFile(filetypes.PixeFile):
    """
    Image files
    """
    NAME = 'ImageFile'
    FILE_EXTENSIONS = ["jpg", "jpeg"]

    def __init__(self, path: str):
        super().__init__(path)

    # helpers
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
                cdate = self.ERROR_DATE

            return cdate
