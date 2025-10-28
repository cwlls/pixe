import pathlib
import datetime
import logging

import pytest
from click.testing import CliRunner
import PIL.ExifTags

import pixe
import filetypes

logging.basicConfig()
LOGGER = logging.getLogger(__name__)
# LOGGER.setLevel(logging.DEBUG)


@pytest.fixture
def runner():
    return CliRunner()


@pytest.fixture
def pixe_file():
    return filetypes.factory


@pytest.fixture
def src_file(src_path):
    return src_path.joinpath("red.jpg")


def test_single_file(runner, src_file, dst_path):
    dest_file = pathlib.Path(dst_path).joinpath(
        "2020", "03-Mar", "20200321_031312_1cdef99be68dbdea159ec6fa8469b41ca13e9e6f.jpg"
    )

    results = runner.invoke(pixe.main.cli, f"--dest {dst_path} {src_file}")
    LOGGER.debug(results.output)

    assert results.exit_code == 0
    assert dest_file.exists()


def test_single_file_no_exist(runner, dst_path):
    results = runner.invoke(pixe.main.cli, f"--dest {dst_path} this/file/really/does/not/exist")

    assert results.exit_code == 2


def test_single_file_bad(runner, dst_path):
    results = runner.invoke(pixe.main.cli, f"--dest {dst_path} /dev/zero")

    assert results.exit_code == 2


@pytest.mark.freeze_time(datetime.datetime.now())
def test_single_file_duplicate(runner, src_file, dst_path):
    import_time = datetime.datetime.now()
    dest_file = dst_path.joinpath(
        "dups",
        import_time.strftime("%Y%m%d_%H%M%S"),
        "2020",
        "20200321_031312_1cdef99be68dbdea159ec6fa8469b41ca13e9e6f.jpg",
    )
    runner.invoke(pixe.main.cli, f"--dest {dst_path} {src_file}")
    results = runner.invoke(pixe.main.cli, f"--dest {dst_path} {src_file}")

    assert results.exit_code == 0
    assert dest_file.exists()


def test_single_file_move(runner, src_path, dst_path):
    src_file = src_path.joinpath("dark/darkred.jpg")
    dest_file = dst_path.joinpath("2021", "06-Jun", "20210614_053114_0e6a2ef81906c98e4735fc9aad4bc6adaae42669.jpg")

    results = runner.invoke(pixe.main.cli, f"--move --dest {dst_path} {src_file}")

    assert results.exit_code == 0
    assert dest_file.exists()
    assert not src_file.exists()


def test_single_file_copy_tagged(runner, src_path, dst_path):
    src_file = src_path.joinpath("dark/darkturquoise.jpg")
    dst_file = dst_path.joinpath("2020", "12-Dec", "20201209_015501_a810b8552a4acf4e13164a74aab3016e583cc93e.jpg")
    src_file_obj = filetypes.factory.get_file_obj(src_file)
    dst_file_obj = filetypes.factory.get_file_obj(dst_file)

    results = runner.invoke(pixe.main.cli, f"--copy --owner 'Joe User' --dest {dst_path} {src_file}")
    src_exif = src_file_obj.metadata
    dst_exif = dst_file_obj.metadata
    old_checksum = src_file_obj.checksum
    new_checksum = dst_file_obj.checksum

    assert results.exit_code == 0
    assert dst_file.exists()
    assert src_exif != dst_exif, "exif tags match!"
    assert dst_exif[PIL.ExifTags.Base.CameraOwnerName] == "Joe User", "owner tag not changed" # fmt: skip
    # assert dst_exif["0th"][33432] == b"Copyright 2020 Joe User."
    assert old_checksum == new_checksum


def test_files_parallel(runner, src_path, dst_path):
    src_files_path = src_path.joinpath("dark")
    num_src_files = 0
    for f in src_files_path.glob("[!.]*"):
        if f.is_file():
            num_src_files += 1
    LOGGER.debug(num_src_files)

    results = runner.invoke(pixe.main.cli, f"--move --dest {dst_path} {src_files_path}")
    LOGGER.debug(results.output)

    num_dst_files = 0
    for f in dst_path.rglob("*"):
        if f.is_file():
            num_dst_files += 1
    LOGGER.debug(num_dst_files)

    assert results.exit_code == 0
    assert num_src_files == num_dst_files
    assert dst_path.joinpath("2022", "05-May", "20220502_182901_6245c093f3e3d43d190688bed0cfd78127c8f799.jpg").exists()


def test_files_serial(runner, src_path, dst_path):
    src_files_path = src_path.joinpath("light")
    num_src_files = 0
    for f in src_files_path.glob("[!.]*"):
        if f.is_file():
            num_src_files += 1
    LOGGER.debug(num_src_files)

    results = runner.invoke(pixe.main.cli, f"--serial --move --dest {dst_path} {src_files_path}")
    LOGGER.debug(results.output)

    num_dst_files = 0
    for f in dst_path.rglob("*"):
        if f.is_file():
            num_dst_files += 1
    LOGGER.debug(num_dst_files)

    assert results.exit_code == 0
    assert num_src_files == num_dst_files
    assert dst_path.joinpath("2018", "10-Oct", "20181029_173758_2322bd54435c6e5587922f581c3584d8f306df1b.jpg").exists()


def test_files_recurse(runner, src_path, dst_path):
    num_src_files = 0
    for f in src_path.rglob("[!.]*"):
        if f.is_file():
            num_src_files += 1
    LOGGER.debug(num_src_files)

    results = runner.invoke(pixe.main.cli, f"--recurse --move --dest {dst_path} {src_path}")
    LOGGER.debug(results.output)

    num_dst_files = 0
    for f in dst_path.rglob("[!.]*"):
        if f.is_file():
            num_dst_files += 1
    LOGGER.debug(num_dst_files)

    assert results.exit_code == 0
    assert num_src_files == num_dst_files
    assert dst_path.joinpath("2022", "02-Feb", "20220226_001821_476bf667385499407e1405f5909f88875dab1873.jpg").exists()
    assert dst_path.joinpath("2018", "03-Mar", "20180319_100139_b78473e5c10d8fd945dd1eee9da7a82320d464d1.jpg").exists()
    assert dst_path.joinpath("2021", "12-Dec", "20211222_153825_d05cae67991384d221e95ae8b30994ce186695ed.jpg").exists()
