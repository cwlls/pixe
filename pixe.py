import hashlib
import io
import datetime
import pathlib
import multiprocessing
import shutil
import re
import os
import fnmatch

import click
import PIL.Image
import piexif

# Using a date that shouldn't appear in our collection, but that also isn't a common default.
# In this case, Ansel Adams birthday.
ERROR_DATE = datetime.datetime(1902, 2, 20)

# store a datetime of when this run began
START_TIME = datetime.datetime.now()


def _calc_checksum(image_path: pathlib.Path, block_size: int = 8192) -> str:
    """
    Create a sha1 checksum of just the image data (no meta/exif).

    :param image_path: a path to an image to process
    :param block_size: the block size to use when chunking up the image data
    :return: a calculated hex digest
    """
    hasher = hashlib.sha1()
    img_io = io.BytesIO()

    # open the image file and save the image data portion as an io.BytesIO object
    with PIL.Image.open(image_path) as im:
        im.save(img_io, im.format)

    # reset the cursor
    img_io.seek(0)

    # chunk_size at a time, update our hash until complete
    while chunk := img_io.read(block_size):
        hasher.update(chunk)

    return hasher.hexdigest()


def _extract_date(image_path: pathlib.Path) -> datetime.datetime:
    """
    Extract the file creation date from EXIF information.

    :param image_path: the path to a specific image file
    :return: a datetime object representing the creation date of the image
    """
    # TODO: use piexif: exif_dict['Exif'][piexif.ExifIFD.DateTimeOriginal]
    with PIL.Image.open(image_path, "r") as im:
        try:
            # attempt to extract the creation date from EXIF tag 36867
            exif = im._getexif()
            cdate = datetime.datetime.strptime(exif[36867], "%Y:%m:%d %H:%M:%S")

        # the requested tag doesn't exist, use the ERROR_DATE global to signify such
        except KeyError:
            cdate = ERROR_DATE

        return cdate


def _new_tags(image_path: pathlib.Path, **kwargs) -> bytes:
    """
    Insert tags into image exif
    :param image_path: path to a given image
    :param owner: the camera owner
    :param copyright: a copyright string
    :return: a bytes object containing the new exif data
    """
    path_str = str(image_path)
    exif_dict = piexif.load(path_str)

    if "owner" in kwargs and kwargs.get("owner") != '':
        exif_dict["Exif"][0xa430] = kwargs.get("owner").encode("ascii")

    if "copyright" in kwargs and kwargs.get("copyright") != '':
        exif_dict["0th"][piexif.ImageIFD.Copyright] = kwargs.get("copyright").encode("ascii")

    return piexif.dump(exif_dict)


def _process_file(file_path: pathlib.Path, dest_str: str, move: bool = False, **kwargs) -> str:
    """
    process a single file
    :param file_path: the path to the file to process
    :param dest_str: where should the processed files be moved/copied to
    :param move: should the file be moved or copied
    :param kwargs: additional options (likely exif tags)
    :return: a string representing the operation that has been performed
    """
    cdate = _extract_date(file_path)
    cdate_str = cdate.strftime("%Y%m%d_%H%M%S")
    hash_str = _calc_checksum(file_path)
    filename = file_path.with_stem(f"{cdate_str}_{hash_str}").with_suffix(file_path.suffix.lower())
    dest_path = pathlib.Path(dest_str).joinpath(str(cdate.year), str(cdate.month))

    # if a similarly named file exists at the destination it means we have a duplicate file
    # prepend 'dups' and the START_TIME of this move process to the destination filepath
    if dest_path.joinpath(filename.name).exists():
        dest_path = pathlib.Path(dest_str).joinpath(
            f"dups/{START_TIME.strftime('%Y%m%d_%H%M%S')}",
            str(cdate.year),
            str(cdate.month),
        )
    dest_path.mkdir(parents=True, exist_ok=True)
    dest_file = dest_path.joinpath(filename.name)

    if move:
        shutil.move(file_path, dest_file)
    else:
        shutil.copy(file_path, dest_file)

    # pass **kwargs to _new_tags so that known tags can be inserted
    # into the file at its destination so we don't muck up the src_file
    # if a copy operation has been requested.
    piexif.insert(_new_tags(dest_file, **kwargs), str(dest_file))

    # return a string showing what file has been moved, so it can be displayed
    return f"{file_path} -> {dest_path.joinpath(filename.name)}"


def parallel_process_files(file_list: list, dest: str, move: bool, **kwargs):
    """
    process a list of files in parallel
    :param file_list: a list of files to be processed
    :param dest: the destination for the process operation
    :param move: is this a move or copy operation
    :param kwargs: additional options (likely exif tags)
    """
    pool = multiprocessing.Pool()
    for file in file_list:
        pool.apply_async(
            _process_file,
            args=(file, dest, move),
            kwds=kwargs,
            callback=(lambda res: print(res, flush=True)),
        )
    pool.close()
    pool.join()


def serial_process_files(file_list: list, dest: str, move: bool, **kwargs):
    """
    process a list of files serially
    :param file_list: a list of files to be processed
    :param dest: the destination for the process operation
    :param move: is this a move or copy operation
    :param kwargs: additional options (likely exif tags)
    """
    for file in file_list:
        print(_process_file(file, dest, move, **kwargs))


@click.command()
@click.argument("src")
@click.option("--dest", "-d", default=".", help="desired destination")
@click.option(
    "--recurse",
    "-r",
    is_flag=True,
    default=False,
    help="recurse into sub-directories (default: off)",
)
@click.option(
    "--parallel/--serial",
    default=True,
    help="process files in parallel (default: --parallel)",
)
@click.option(
    "--move/--copy",
    "--mv/--cp",
    is_flag=True,
    default=False,
    help="move files into DEST rather than copying (default: --copy)",
)
@click.option(
    "--owner",
    default='',
    help="add camera owner to exif tags",
)
@click.option(
    "--copyright",
    default='',
    help="add copyright string to exif tags"
)
def cli(src: str, dest: str, recurse: bool, parallel: bool, move: bool, **kwargs):
    file_path = pathlib.Path(src)
    # jpg pattern (case insensitive)
    file_re = re.compile(fnmatch.translate("*.jpg"), re.IGNORECASE)
    if file_path.exists():
        if file_path.is_dir():
            file_list = []
            for root, dirs, files in os.walk(file_path, topdown=True):
                for file in files:
                    if file_re.match(file):
                        file_list.append(pathlib.Path(root).joinpath(file))
                if not recurse:
                    break

            if parallel:
                parallel_process_files(file_list, dest, move, **kwargs)
            else:
                serial_process_files(file_list, dest, move, **kwargs)

        elif file_path.is_file():
            print(_process_file(file_path, dest, move, **kwargs))
        else:
            raise click.exceptions.BadParameter(src)
    else:
        raise click.exceptions.BadParameter(src)
