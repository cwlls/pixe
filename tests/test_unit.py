import pathlib
import datetime
import os

import pytest
import piexif

import pixe


@pytest.fixture
def src_file(src_path):
    return src_path.joinpath("red.jpg")


def test_calc_checksum(src_file):
    expected_checksum = "1cdef99be68dbdea159ec6fa8469b41ca13e9e6f"

    calculated_checksum = pixe._calc_checksum(src_file)

    assert expected_checksum == calculated_checksum


def test_extract_date(src_file):
    expected_date = datetime.datetime(2020, 3, 21, 3, 13, 12)

    extracted_date = pixe._extract_date(src_file)

    assert expected_date == extracted_date


def test_new_tag_owner(src_file):
    path_str = str(src_file)
    orig_exif = piexif.load(path_str)

    new_exif = piexif.load(pixe._new_tags(src_file, owner="Joe User"))

    assert orig_exif != new_exif
    assert new_exif["Exif"][0xa430] == b"Joe User"


def test_new_tag_copyright(src_file):
    path_str = str(src_file)
    orig_exif = piexif.load(path_str)

    new_exif = piexif.load(
        pixe._new_tags(src_file, copyright="Copyright 2023 Joe User.")
    )

    assert orig_exif != new_exif
    assert new_exif["0th"][piexif.ImageIFD.Copyright] == b"Copyright 2023 Joe User."


def test_process_file(src_file, dst_path):
    expected_file = pathlib.Path(dst_path).joinpath(
        "2020", "3", "20200321_031312_1cdef99be68dbdea159ec6fa8469b41ca13e9e6f.jpg"
    )

    pixe._process_file(src_file, dst_path)

    assert expected_file.exists()


def test_process_file_no_date(src_path, dst_path):
    src_file = src_path.joinpath("chocolate.jpg")
    new_file = pathlib.Path(dst_path).joinpath(
        "1902", "2", "19020220_000000_2a00d2b48e39f63cf834d4f7c50b2c1aa3b43a9c.jpg"
    )

    pixe._process_file(src_file, dst_path)

    assert new_file.exists()
