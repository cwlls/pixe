import pathlib
import datetime
import os

import pytest

import tomte


@pytest.fixture
def src_path():
    return pathlib.Path(os.path.dirname(os.path.realpath(__file__))).joinpath('data/img')


@pytest.fixture
def src_file(src_path):
    return src_path.joinpath('chocolate.jpg')


def test_calc_checksum(src_file):
    expected_checksum = '9c12b09015e8fe1bdd3c9aa765d08c5cdd60a485'

    calculated_checksum = tomte._calc_checksum(src_file)

    assert expected_checksum == calculated_checksum


def test_extract_date(src_file):
    expected_date = datetime.datetime(2021, 6, 28, 9, 31, 21)

    extracted_date = tomte._extract_date(src_file)

    assert expected_date == extracted_date


def test_process_file(src_file, tmp_path):
    expected_file = pathlib.Path(tmp_path).joinpath('2021/6')

    tomte._process_file(src_file, tmp_path)

    assert expected_file.exists()


def test_process_file_no_date(src_path, tmp_path):
    src_file = src_path.joinpath('no_date.jpg')
    new_file = pathlib.Path(tmp_path).joinpath('1902', '2', '19020220_000000_9c12b09015e8fe1bdd3c9aa765d08c5cdd60a485.jpg')

    tomte._process_file(src_file, tmp_path)

    assert new_file.exists()
