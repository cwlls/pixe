import pathlib
import datetime

import tomte


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
