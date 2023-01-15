import pathlib

import pytest

import tomte


def test_process_file(tmp_path):
    test_file = pathlib.Path('data/img/chocolate.jpg')
    new_file = pathlib.Path(tmp_path).joinpath('2021', '6', '20210628_093121_9c12b09015e8fe1bdd3c9aa765d08c5cdd60a485.jpg')

    tomte._process_file(test_file, tmp_path)

    assert new_file.exists()


def test_process_file_no_date(tmp_path):
    test_file = pathlib.Path('data/img/no_date.jpg')
    new_file = pathlib.Path(tmp_path).joinpath('1902', '2', '19020220_000000_9c12b09015e8fe1bdd3c9aa765d08c5cdd60a485.jpg')

    tomte._process_file(test_file, tmp_path)

    assert new_file.exists()


@pytest.fixture
def dest_path(tmp_path):
    return pathlib.Path(tmp_path)


@pytest.fixture
def src_path():
    return pathlib.Path('data/img')


@pytest.fixture
def file_list(src_path):
    file_list = []
    for img in src_path.glob('*.jpg'):
        file_list.append(img)

    return file_list


def test_parallel_process_files(src_path, dest_path, file_list):
    tomte.parallel_process_files(file_list, dest_path)

    assert dest_path.joinpath('1902/2/19020220_000000_9c12b09015e8fe1bdd3c9aa765d08c5cdd60a485.jpg').exists()
    assert dest_path.joinpath('2021/10/20211013_145024_193618a9c1dad0600f5d571268404d73d5a16173.jpg').exists()
    assert dest_path.joinpath('2022/9/20220928_020550_141e2b9762f60952aa0510ac9309a4ae6126b817.jpg').exists()


def test_serial_process_files(src_path, dest_path, file_list):
    tomte.serial_process_files(file_list, dest_path)

    assert dest_path.joinpath('1902/2/19020220_000000_9c12b09015e8fe1bdd3c9aa765d08c5cdd60a485.jpg').exists()
    assert dest_path.joinpath('2021/10/20211013_145024_193618a9c1dad0600f5d571268404d73d5a16173.jpg').exists()
    assert dest_path.joinpath('2022/9/20220928_020550_141e2b9762f60952aa0510ac9309a4ae6126b817.jpg').exists()
