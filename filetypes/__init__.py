import typing
import pathlib
import datetime


class PixeFile:

    def __init__(self):
        self._filetypes = {}

    def register_filetype(self, extension: str, creator: typing.Callable):
        self._filetypes[extension] = creator

    def get_file_obj(self, path: pathlib.Path) -> typing.Callable:
        if creator := self._filetypes.get(path.suffix.lower().lstrip('.')):
            return creator(path)
        else:
            raise ValueError


# Using a date that shouldn't appear in our collection, but that also isn't a common default.
# In this case, Ansel Adams birthday.
ERROR_DATE = datetime.datetime(1902, 2, 20)

factory = PixeFile()
from . import image_file

# class PixeFile:
#     """
#     A base class for supported file types in Pixe
#     """
#
#     FILE_EXTENSIONS = []
#     NAME = 'PixeFile'
#     ERROR_DATE = datetime.datetime(1902, 2, 20)
#
#     def __init__(self, path: str):
#         self.path = pathlib.Path(path)
#
#     def __repr__(self):
#         return f"{self.NAME}: {self.path}"
#
#     @classmethod
#     def supported_extension(cls, extension: str) -> bool:
#         return extension in cls.FILE_EXTENSIONS
#
#     @classmethod
#     def create_file(cls, path: str):
#         return cls.__init__(cls, path)
#
#     # helper functions
#     @property
#     def creation_date(self) -> datetime.datetime:
#         raise NotImplementedError
#
#     @property
#     def checksum(self, block_size: int = 8192) -> str:
#         raise NotImplementedError
#
#     @classmethod
#     def add_metadata(cls, file: pathlib.Path, **kwargs):
#         """
#         Add a metadata tag to a given file.
#
#         :param file: the file to be acted upon
#         :param tag: a supported tag name
#         :param value: a string to be written to tag
#         """
#         raise NotImplementedError
