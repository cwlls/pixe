import hashlib
import io
import datetime
import pathlib
import multiprocessing
import shutil

import click
import PIL.Image

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

    # open the image file and save the image data portion as a io.BytesIO object
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
    with PIL.Image.open(image_path, "r") as im:
        try:
            # attempt to extract the creation date from EXIF tag 36867
            exif = im._getexif()
            cdate = datetime.datetime.strptime(exif[36867], "%Y:%m:%d %H:%M:%S")

        # the requested tag doesn't exist, use the ERROR_DATE global to signify such
        except KeyError:
            cdate = ERROR_DATE

        return cdate


def _process_file(file_path: pathlib.Path, dest_str: str, move: bool):
    cdate = _extract_date(file_path)
    cdate_str = cdate.strftime("%Y%m%d_%H%M%S")
    hash_str = _calc_checksum(file_path)
    filename = file_path.with_stem(f"{cdate_str}_{hash_str}").with_suffix(
        file_path.suffix.lower()
    )
    dest_path = pathlib.Path(dest_str).joinpath(str(cdate.year), str(cdate.month))

    if dest_path.joinpath(filename.name).exists():
        dest_path = pathlib.Path(dest_str).joinpath(
            f"dups/{START_TIME.strftime('%Y%m%d_%H%M%S')}",
            str(cdate.year),
            str(cdate.month),
        )
    dest_path.mkdir(parents=True, exist_ok=True)

    if move:
        shutil.move(file_path, dest_path.joinpath(filename.name))
    else:
        shutil.copy(file_path, dest_path.joinpath(filename.name))

    return f"{file_path} -> {dest_path.joinpath(filename.name)}"


def parallel_process_files(file_list: list, dest: str, move: bool):
    pool = multiprocessing.Pool()
    for file in file_list:
        pool.apply_async(
            _process_file,
            args=(file, dest, move),
            callback=(lambda res: print(res, flush=True)),
        )
    pool.close()
    pool.join()


def serial_process_files(file_list: list, dest: str, move: bool):
    for file in file_list:
        print(_process_file(file, dest, move))


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
    "--parallel/--no-parallel",
    default=True,
    help="process files in parallel (default: --parallel)",
)
@click.option(
    "--move",
    "--mv",
    is_flag=True,
    default=False,
    help="move files into DEST rather than copying (default: copy)",
)
def cli(src: str, dest: str, recurse: bool, parallel: bool, move: bool):
    file_path = pathlib.Path(src)
    if file_path.exists():
        if file_path.is_dir():
            file_list = []
            if recurse:
                for img in file_path.rglob("*.jpg"):
                    file_list.append(img)
            else:
                for img in file_path.glob("*.jpg"):
                    file_list.append(img)

            if parallel:
                parallel_process_files(file_list, dest, move)
            else:
                serial_process_files(file_list, dest, move)

        elif file_path.is_file():
            print(_process_file(file_path, dest, move))
        else:
            raise click.exceptions.BadParameter(src)
    else:
        raise click.exceptions.BadParameter(src)
