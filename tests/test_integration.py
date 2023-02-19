import pathlib
import os
import datetime

import pytest
from click.testing import CliRunner
import piexif

import pixe


@pytest.fixture
def runner():
    return CliRunner()


@pytest.fixture
def src_file(src_path):
    return src_path.joinpath("red.jpg")


def test_single_file(runner, src_file, dst_path):
    dest_file = pathlib.Path(dst_path).joinpath(
        "2020", "3", "20200321_031312_1cdef99be68dbdea159ec6fa8469b41ca13e9e6f.jpg"
    )

    results = runner.invoke(pixe.cli, f"--dest {dst_path} {src_file}")

    assert results.exit_code == 0
    assert dest_file.exists()


def test_single_file_no_exist(runner, dst_path):
    results = runner.invoke(pixe.cli, f"--dest {dst_path} this/file/really/does/not/exist")

    assert results.exit_code == 2


def test_single_file_bad(runner, dst_path):
    results = runner.invoke(pixe.cli, f"--dest {dst_path} /dev/zero")

    assert results.exit_code == 2


def test_single_file_duplicate(runner, src_file, dst_path):
    import_time = datetime.datetime.now()
    dest_file = dst_path.joinpath(
        "dups",
        import_time.strftime("%Y%m%d_%H%M%S"),
        "2020",
        "3",
        "20200321_031312_1cdef99be68dbdea159ec6fa8469b41ca13e9e6f.jpg",
    )
    runner.invoke(pixe.cli, f"--dest {dst_path} {src_file}")
    results = runner.invoke(pixe.cli, f"--dest {dst_path} {src_file}")

    assert results.exit_code == 0
    assert dest_file.exists()


def test_single_file_move(runner, src_file, dst_path):
    dest_file = dst_path.joinpath("2020", "3", "20200321_031312_1cdef99be68dbdea159ec6fa8469b41ca13e9e6f.jpg")

    results = runner.invoke(pixe.cli, f"--move --dest {dst_path} {src_file}")

    assert results.exit_code == 0
    assert dest_file.exists()
    assert not src_file.exists()


def test_single_file_copy_tagged(runner, src_path, dst_path):
    src_file = src_path.joinpath("dark/darkturquoise.jpg")
    dest_file = dst_path.joinpath("2020", "12", "20201209_015501_a810b8552a4acf4e13164a74aab3016e583cc93e.jpg")
    old_checksum = dest_file.stem.split("_")[2]

    results = runner.invoke(
        pixe.cli, f"--copy --owner 'Joe User' --copyright 'Copyright 2020 Joe User.' --dest {dst_path} {src_file}"
    )
    src_exif = piexif.load(str(src_file))
    dst_exif = piexif.load(str(dest_file))
    new_checksum = pixe._calc_checksum(dest_file)

    assert results.exit_code == 0
    assert dest_file.exists()
    assert src_exif != dst_exif
    assert dst_exif["Exif"][0xA430] == b"Joe User"
    assert dst_exif["0th"][piexif.ImageIFD.Copyright] == b"Copyright 2020 Joe User."
    assert old_checksum == new_checksum


def test_files_parallel(runner, src_path, dst_path):
    results = runner.invoke(pixe.cli, f"--dest {dst_path} {src_path}")

    assert results.exit_code == 0
    assert dst_path.joinpath("2021", "12", "20211222_153825_d05cae67991384d221e95ae8b30994ce186695ed.jpg").exists()


def test_files_serial(runner, src_path, dst_path):
    results = runner.invoke(pixe.cli, f"--serial --dest {dst_path} {src_path}")

    assert results.exit_code == 0
    assert dst_path.joinpath("2021", "12", "20211222_153825_d05cae67991384d221e95ae8b30994ce186695ed.jpg").exists()


def test_files_recurse(runner, src_path, dst_path):
    results = runner.invoke(pixe.cli, f"--recurse --dest {dst_path} {src_path}")

    assert results.exit_code == 0
    assert dst_path.joinpath("2022", "2", "20220226_001821_476bf667385499407e1405f5909f88875dab1873.jpg").exists()
    assert dst_path.joinpath("2018", "3", "20180319_100139_b78473e5c10d8fd945dd1eee9da7a82320d464d1.jpg").exists()
    assert dst_path.joinpath("2021", "12", "20211222_153825_d05cae67991384d221e95ae8b30994ce186695ed.jpg").exists()
