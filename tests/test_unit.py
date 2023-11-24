import pathlib
import datetime
import os

import pytest
import piexif

import pixe
import filetypes

global filetypes
filetypes.APP = pixe.PixeApp()
import filetypes.image_file


@pytest.fixture
def src_img_file(src_path):
    return filetypes.image_file.ImageFile(src_path.joinpath("red.jpg"))


@pytest.fixture
def src_img_file_path(src_path):
    return pathlib.Path(src_path.joinpath("red.jpg"))


def test_img_file_checksum(src_img_file):
    expected_checksum = "1cdef99be68dbdea159ec6fa8469b41ca13e9e6f"

    calculated_checksum = src_img_file.checksum

    assert expected_checksum == calculated_checksum


def test_img_file_create_date(src_img_file):
    expected_date = datetime.datetime(2020, 3, 21, 3, 13, 12)

    extracted_date = src_img_file.creation_date

    assert expected_date == extracted_date


def test_img_file_add_metadata_owner(src_img_file_path):
    path_str = str(src_img_file_path)
    orig_exif = piexif.load(path_str)

    new_exif = piexif.load(filetypes.image_file.ImageFile.add_metadata(src_img_file_path,
                                                                       'owner',
                                                                       'Joe User'))
    assert orig_exif != new_exif
    assert new_exif["Exif"][0xa430] == b"Joe User"


def test_img_file_add_metadata_copyright(src_img_file_path):
    path_str = str(src_img_file_path)
    orig_exif = piexif.load(path_str)

    new_exif = piexif.load(filetypes.image_file.ImageFile.add_metadata(src_img_file_path,
                                                                       'copyright',
                                                                       'Copyright 2023 Joe User.'))

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
