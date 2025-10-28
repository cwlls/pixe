import pathlib
import datetime
import logging

import pytest
import piexif

import pixe.main
import filetypes

LOGGER = logging.getLogger(__name__)


@pytest.fixture
def src_img(sandbox):
    return filetypes.factory.get_file_obj(sandbox.joinpath("src", "red.jpg"))


@pytest.fixture
def src_img_path(sandbox):
    return sandbox.joinpath("src", "red.jpg")


def test_img_file_checksum(src_img):
    expected_checksum = "1cdef99be68dbdea159ec6fa8469b41ca13e9e6f"

    calculated_checksum = src_img.checksum

    assert expected_checksum == calculated_checksum


def test_img_file_create_date(src_img):
    expected_date = datetime.datetime(2020, 3, 21, 3, 13, 12)

    extracted_date = src_img.creation_date

    assert expected_date == extracted_date


def test_img_file_add_metadata_owner(src_img_path):
    path_str = str(src_img_path)
    orig_exif = piexif.load(path_str)
    file_obj = filetypes.factory.get_file_obj(src_img_path)

    file_obj.add_metadata(src_img_path, owner="Joe User")
    new_exif = piexif.load(str(src_img_path))

    assert orig_exif != new_exif
    assert new_exif["Exif"][0xA430] == b"Joe User"


## REMOVED UNTIL COPYRIGHT TAGS ARE ALLOWED
# def test_img_file_add_metadata_copyright(src_img_file_path):
#     path_str = str(src_img_file_path)
#     orig_exif = piexif.load(path_str)
#     file_obj = filetypes.factory.get_file_obj(src_img_file_path)

#     file_obj.add_metadata(src_img_file_path, copyright="Copyright 2023 Joe User.")
#     new_exif = piexif.load(str(src_img_file_path))

#     assert orig_exif != new_exif
#     assert new_exif["0th"][piexif.ImageIFD.Copyright] == b"Copyright 2023 Joe User."


def test_process_file(src_img, dst_path):
    expected_file = pathlib.Path(dst_path).joinpath(
        "2020", "03-Mar", "20200321_031312_1cdef99be68dbdea159ec6fa8469b41ca13e9e6f.jpg"
    )

    pixe.main.process_file(src_img, dst_path)

    assert expected_file.exists()


def test_process_img_file_no_date(src_path, dst_path):
    src_file = src_path.joinpath("chocolate.jpg")
    new_file = pathlib.Path(dst_path).joinpath(
        "1902", "02-Feb", "19020220_000000_2a00d2b48e39f63cf834d4f7c50b2c1aa3b43a9c.jpg"
    )
    test_file = filetypes.image_file.ImageFile(src_file)

    pixe.main.process_file(test_file, dst_path)

    assert new_file.exists()
