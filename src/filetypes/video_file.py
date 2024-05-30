import logging
import datetime
import hashlib
import io
import pathlib
import sys

import ffmpeg

import filetypes
from . import base

FACTORY = filetypes.factory
LOGGER = logging.getLogger(__name__)


class VideoFile(base.PixeFile):
    """
    Video files
    """

    EXTENSIONS = ["mp4", "m4v", "mov"]
    ALLOWED_TAGS = []

    def __init__(self, path: pathlib.Path):
        super().__init__(path)

    @property
    def checksum(self, block_size: int = 8192) -> str:
        try:
            file = ffmpeg.input(self.path)
            file = ffmpeg.output(file, "-", f="hash", hash="SHA160")
            chksum = ffmpeg.run(file, capture_stdout=True, capture_stderr=True)
            chksum = chksum[0].decode().lstrip("SHA160=").rstrip()
            LOGGER.debug(f"CHECKSUM: {chksum}")
        except ffmpeg.Error as e:
            LOGGER.error(f"{e.stderr}")
            sys.exit(1)

        return chksum

    @property
    def creation_date(self) -> datetime.datetime:
        try:
            probe = ffmpeg.probe(self.path)
        except ffmpeg.Error as e:
            LOGGER.error(f"{e.stderr}")
            sys.exit(1)

        if cdate := probe["format"]["tags"]["creation_time"]:
            return datetime.datetime.strptime(cdate, "%Y-%m-%dT%H:%M:%S.%fZ")
        else:
            return self.DEFAULT_DATE

    # @property
    # def metadata(self):
    #     pass

    # @classmethod
    # def add_metadata(cls, file: pathlib.Path, **kwargs):
    #     assert file.suffix.lstrip('.').lower() in cls.EXTENSIONS
    #     # for key in kwargs.keys():
    #     #     assert key in cls.ALLOWED_TAGS


# add ImageFile extensions and creator method to the Factory
for ext in VideoFile.EXTENSIONS:
    FACTORY.register_filetype(ext, VideoFile)
