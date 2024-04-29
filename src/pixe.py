import datetime
import pathlib
import multiprocessing
import shutil
import os
import logging
import time

import click

import filetypes

# create global reference to filetypes factory
PIXE_FILE = filetypes.factory

# setup logging
LOGGER = logging.getLogger(__name__)

# store a datetime of when this run began
START_TIME = datetime.datetime.now()

__version__ = "0.7.2"


def process_file(file: filetypes, dest_str: str, move: bool = False, **kwargs) -> str:
    """
    process a single file
    :param file: a file to process
    :param dest_str: where should the processed files be moved/copied to
    :param move: should the file be moved or copied
    :param kwargs: additional options (likely exif tags)
    :return: a string representing the operation that has been performed
    """
    cdate = file.creation_date
    cdate_str = cdate.strftime("%Y%m%d_%H%M%S")
    hash_str = file.checksum
    filename = file.path.with_stem(f"{cdate_str}_{hash_str}").with_suffix(
        file.path.suffix.lower()
    )
    dest_path = pathlib.Path(dest_str).joinpath(
        str(cdate.year), str(cdate.strftime("%m-%b"))
    )

    # if a similarly named file exists at the destination it means we have a duplicate file
    # prepend 'dups' and the START_TIME of this move process to the destination filepath
    if dest_path.joinpath(filename.name).exists():
        dest_path = pathlib.Path(dest_str).joinpath(
            f"dups/{START_TIME.strftime('%Y%m%d_%H%M%S')}",
            str(cdate.year),
        )
    dest_path.mkdir(parents=True, exist_ok=True)
    dest_file = dest_path.joinpath(filename.name)

    if move:
        shutil.move(file.path, dest_file)
    else:
        shutil.copy(file.path, dest_file)

    # pass **kwargs to add_metadata() so that known tags can be inserted
    # into the file at its destination, so we don't muck up the src_file
    # if a copy operation has been requested.
    file.add_metadata(dest_file, **kwargs)

    # return a string showing what file has been moved, so it can be displayed
    return f"{file.path} -> {dest_path.joinpath(filename.name)}"


def parallel_process_files(file_list: list[filetypes], dest: str, move: bool, **kwargs):
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
            process_file,
            args=(file, dest, move),
            kwds=kwargs,
            callback=(lambda res: print(res, flush=True)),
            error_callback=(lambda res: print(res, flush=True)),
        )
    pool.close()
    pool.join()


def serial_process_files(file_list: list[filetypes], dest: str, move: bool, **kwargs):
    """
    process a list of files serially
    :param file_list: a list of files to be processed
    :param dest: the destination for the process operation
    :param move: is this a move or copy operation
    :param kwargs: additional options (likely exif tags)
    """
    for file in file_list:
        print(process_file(file, dest, move, **kwargs))


@click.command()
@click.argument("src")
@click.version_option(__version__, "-v", "--version")
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
    default="",
    help="add camera owner to exif tags",
)
# TODO: waiting on implementation in the image_file class
# @click.option("--copyright", default="", help="add copyright string to exif tags")
def cli(src: str, dest: str, recurse: bool, parallel: bool, move: bool, **kwargs):
    start_time = time.perf_counter()
    file_count = 0

    file_path = pathlib.Path(src)
    if file_path.exists():
        if file_path.is_dir():
            file_list = []
            for root, dirs, files in os.walk(file_path, topdown=True):
                for file in files:
                    if PIXE_FILE.get_ext_regex().match(file):
                        file_count += 1
                        file_list.append(PIXE_FILE.get_file_obj(pathlib.Path(file)))
                if not recurse:
                    break

            if parallel:
                parallel_process_files(file_list, dest, move, **kwargs)
            else:
                serial_process_files(file_list, dest, move, **kwargs)

        elif file_path.is_file():
            file_count = 1
            print(process_file(PIXE_FILE.get_file_obj(file_path), dest, move, **kwargs))
        else:
            raise click.exceptions.BadParameter(src)
    else:
        raise click.exceptions.BadParameter(src)

    end_time = time.perf_counter()
    print(
        f"------------------------\nprocessed {file_count} files in {(end_time - start_time):.2f}secs"
    )
