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

    with PIL.Image.open(image_path) as im:
        im.save(img_io, im.format)

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
def cli():
    click.echo("Just a test")