import pathlib
import datetime

import tomte


def test_calc_checksum():
    test_file = pathlib.Path('data/img/chocolate.jpg')
    imgdata_sha1sum = '9c12b09015e8fe1bdd3c9aa765d08c5cdd60a485'

    calculated_checksum = tomte._calc_checksum(test_file)

    assert calculated_checksum == imgdata_sha1sum


def test_extract_date():
    test_file = pathlib.Path('data/img/chocolate.jpg')
    date_time_original = datetime.datetime(2021, 6, 28, 9, 31, 21)

    extracted_date = tomte._extract_date(test_file)

    assert extracted_date == date_time_original


def test_no_date():
    test_file = pathlib.Path('data/img/no_date.jpg')
    error_date_time = tomte.ERROR_DATE

    extracted_date = tomte._extract_date(test_file)

    assert extracted_date == error_date_time


def test_process_file(tmp_path):
    test_file = pathlib.Path('data/img/chocolate.jpg')
    new_file = pathlib.Path(tmp_path).joinpath('2021', '6', '20210628_093121_9c12b09015e8fe1bdd3c9aa765d08c5cdd60a485.jpg')

    tomte.process_file(test_file, tmp_path)

    assert new_file.exists()

def test_process_file_no_date(tmp_path):
    test_file = pathlib.Path('data/img/no_date.jpg')
    new_file = pathlib.Path(tmp_path).joinpath('1902', '2', '19020220_000000_9c12b09015e8fe1bdd3c9aa765d08c5cdd60a485.jpg')

    tomte.process_file(test_file, tmp_path)

    assert new_file.exists()
