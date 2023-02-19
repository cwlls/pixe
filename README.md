# pixe
[![flake8](https://github.com/ithuna/pixe/actions/workflows/flake8.yml/badge.svg)](https://github.com/ithuna/pixe/actions/workflows/flake8.yml) [![pytest](https://github.com/ithuna/pixe/actions/workflows/pytest.yml/badge.svg)](https://github.com/ithuna/pixe/actions/workflows/pytest.yml) [![Code style: black](https://img.shields.io/badge/code%20style-black-000000.svg)](https://github.com/psf/black)

A digital helper to keep your files neat and tidy

## Installation
`$ pip install pixe`

## Usage
```
Usage: pixe [OPTIONS] SRC

Options:
  -d, --dest TEXT              desired destination
  -r, --recurse                recurse into sub-directories (default: off)
  --parallel / --serial        process files in parallel (default: --parallel)
  --move, --mv / --copy, --cp  move files into DEST rather than copying
                               (default: --copy)
  --owner TEXT                 add camera owner to exif tags
  --copyright TEXT             add copyright string to exif tags
  --help                       Show this message and exit.
```
