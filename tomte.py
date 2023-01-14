import os
import multiprocessing
import hashlib
import io
import datetime
import pathlib

import click
import PIL.Image

ERROR_DATE = datetime.datetime(1900, 1, 1)


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
    with PIL.Image.open(image_path, 'r') as im:
        im.save(img_io, im.format)

    # chunk_size at a time, update our hash until complete
    while chunk := img_io.read(block_size):
        hasher.update(chunk)

    return hasher.hexdigest()


def _extract_date(im: PIL.Image) -> datetime.datetime:
    try:
        exif = im.getexif()
        cdate = datetime.datetime.strptime(exif[306], '%Y:%m:%d %H:%M:%S')
    except KeyError:
        cdate = ERROR_DATE

    return cdate


@click.command()
@click.argument('src')
@click.option('--dest', default='.', help='desired destination')
def cli(src: str, dest: str):
    file_path = pathlib.Path(src)
    if file_path.exists():
        if file_path.is_dir():
            pass
        elif file_path.is_file():
            hash_str = _calc_checksum(file_path)
            with PIL.Image.open(file_path, 'r') as im:
                cdate = _extract_date(im)

            if cdate == ERROR_DATE:
                # don't rename file
                pass
            else:
                # rename file
                pass

            click.echo(f"{file_path.name} -- {cdate} -- {hash_str}")
        else:
            raise click.exceptions.BadParameter(src)
    else:
        raise click.exceptions.BadParameter(src)
