#
# Package for all Pixe filetypes
# each filetype should use the following pattern to link to the
# main application:
#   import filetypes
#   APP = features.APP
#
import datetime
import pathlib

# This name will be updated once the app has been constructed,
# but before filetypes are loaded.
APP = None


class PixeFile:
    """
    A base class for supported file types in Pixe
    """

    FILE_EXTENSIONS = []
    NAME = 'PixeFile'
    ERROR_DATE = datetime.datetime(1902, 2, 20)

    def __init__(self, path: str):
        # add FILE_EXTENSIONS to the APP
        APP.add_extensions(self.FILE_EXTENSIONS, self.create_file)

        self.path = pathlib.Path(path)

    def __repr__(self):
        return f"{self.NAME}: {self.path}"

    @classmethod
    def supported_extension(cls, extension: str) -> bool:
        return extension in cls.FILE_EXTENSIONS

    @classmethod
    def create_file(cls, path: str):
        return cls.__init__(cls, path)

    # helper functions
    @property
    def creation_date(self) -> datetime.datetime:
        raise NotImplementedError

    @property
    def checksum(self, block_size: int = 8192) -> str:
        raise NotImplementedError

    def _set_metadata(self, tag: str, value: str):
        raise NotImplementedError
