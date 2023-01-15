import pathlib
import os

import pytest
from click.testing import CliRunner

import tomte


@pytest.fixture
def runner():
    return CliRunner()


@pytest.fixture
def src_path():
    return pathlib.Path(os.path.dirname(os.path.realpath(__file__))).joinpath('data/img')


@pytest.fixture
def src_file(src_path):
    return src_path.joinpath('chocolate.jpg')


def test_single_file(runner, src_file, tmp_path):
    dest_file = tmp_path.joinpath('2021/6/20210628_093121_9c12b09015e8fe1bdd3c9aa765d08c5cdd60a485.jpg')

    results = runner.invoke(tomte.cli, f"--dest {tmp_path} {src_file}")

    assert results.exit_code == 0
    assert dest_file.exists()


def test_single_file_no_exist(runner, tmp_path):
    results = runner.invoke(tomte.cli, f"--dest {tmp_path} this/file/really/does/not/exist")

    assert results.exit_code == 2


def test_single_file_bad(runner, tmp_path):
    results = runner.invoke(tomte.cli, f"--dest {tmp_path} /dev/zero")

    assert results.exit_code == 2


def test_files_parallel(runner, src_path, tmp_path):
    results = runner.invoke(tomte.cli, f"--dest {tmp_path} {src_path}")

    assert results.exit_code == 0
    assert tmp_path.joinpath('1902/2/19020220_000000_9c12b09015e8fe1bdd3c9aa765d08c5cdd60a485.jpg').exists()
    assert tmp_path.joinpath('2021/10/20211013_145024_193618a9c1dad0600f5d571268404d73d5a16173.jpg').exists()
    assert tmp_path.joinpath('2022/9/20220928_020550_141e2b9762f60952aa0510ac9309a4ae6126b817.jpg').exists()
    assert not tmp_path.joinpath('2022/3/20220305_225114_93f3c5c0b3d0e23349e238231131b3f297889be4.jpg').exists()


def test_files_serial(runner, src_path, tmp_path):
    results = runner.invoke(tomte.cli, f"--no-parallel --dest {tmp_path} {src_path}")

    assert results.exit_code == 0
    assert tmp_path.joinpath('1902/2/19020220_000000_9c12b09015e8fe1bdd3c9aa765d08c5cdd60a485.jpg').exists()
    assert tmp_path.joinpath('2021/10/20211013_145024_193618a9c1dad0600f5d571268404d73d5a16173.jpg').exists()
    assert tmp_path.joinpath('2022/9/20220928_020550_141e2b9762f60952aa0510ac9309a4ae6126b817.jpg').exists()


def test_files_recurse(runner, src_path, tmp_path):
    results = runner.invoke(tomte.cli, f"--recurse --dest {tmp_path} {src_path}")

    assert results.exit_code == 0
    assert tmp_path.joinpath('2022/3/20220305_225114_93f3c5c0b3d0e23349e238231131b3f297889be4.jpg').exists()
