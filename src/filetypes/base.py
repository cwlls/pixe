import datetime
import typing
import pathlib
import fnmatch
import re


class Factory:

    def __init__(self):
        self._filetypes = {}

    def register_filetype(self, extension: str, creator: typing.Callable):
        self._filetypes[extension] = creator

    def get_file_obj(self, path: pathlib.Path) -> typing.Callable:
        if creator := self._filetypes.get(path.suffix.lower().lstrip('.')):
            return creator(path)
        else:
            raise ValueError

    def get_ext_regex(self) -> re.Pattern:
        lst = [fnmatch.translate(f"*.{ext}") for ext in self._filetypes]
        pattern_str = "|".join(lst)

        return re.compile(pattern_str, re.IGNORECASE)


class PixeFile:
    """
    A base class for other filetypes
    """
    # A date that shouldn't appear in our collection, but that also isn't a common default.
    # In this case, Ansel Adams birthday.
    DEFAULT_DATE = datetime.datetime(1902, 2, 20)

    # A checksum that "looks" like a normal checksum, but that should be easily identifiable
    # as not a sha1sum and easy to see in directory listings.
    DEFAULT_CHECKSUM = 'deadc0de'

    # class variables that should be overridden in child classes
    # a list of file extensions that will be processed by this filetype
    EXTENSIONS = []

    # a list of metadata tags that can be added
    ALLOWED_TAGS = []

    def __init__(self, path: pathlib.Path):
        self.path = path

    @property
    def checksum(self, block_size: int = 8192) -> str:
        """
        Create a sha1 checksum of just the data (no meta/exif).

        :param block_size: the block size to use when chunking up the image data
        :return: a calculated hex digest
        """
        return self.DEFAULT_CHECKSUM

    @property
    def creation_date(self) -> datetime.datetime:
        """
        Extract the file creation date from file metadata and return.

        :return: a datetime object representing the creation date of the file
        """
        return self.DEFAULT_DATE

    @property
    def metadata(self) -> dict:
        """
        Extract the file metadata and return as a dict.

        :return: a dict containing the file metadata
        """
        return {}

    @classmethod
    def add_metadata(cls, file: pathlib.Path, **kwargs) -> bool:
        """
        Add specific metadata, specified in class.ALLOWED_TAGS, to a
        given file. Because metadata will only be added a file once it's
        in the final destination (don't write to src files), this is a class
        method that shouldn't act on the src file specified in self.path, but rather
        the file in its final location.

        :param file: the path to the file to be acted upon
        :param kwargs: keyword argument list of tag/value pairs
        :return: a bool signifying whether the metadata could be added
        """
        return False


# add a reference to the filetypes factory at the top of your filetype
# FACTORY = base.Factory()
#
# add <SUBCLASS NAME> extensions and creator method to the Factory
# for ext in <SUBCLASS_NAME>.EXTENSIONS:
#     FACTORY.register_filetype(ext, <SUBCLASS_NAME>)
