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

### Options

#### -d, --dest TEXT
The base directory of where you want the processed files to end up. If this option is not specified, 
the current working directory will be used.

#### -r, --recurse
`pixe` will recurse into any subdirectories it finds beneath SRC. The default is to not recurse.

#### --parallel / --serial
Should `pixe` process multiple files at once, in parallel using multiprocessing using all 
available cores. If `--serial` is chosen one file will be processed at a time. The default is
to process files in parallel if there is more than one file specified for processing.

#### --move, --mv / --copy, --cp
By default, `pixe` will copy files into DEST and leave the source files untouched. This can be
overridden by specifying `--move`.

#### --owner
A string which will be inserted into the CameraOwnerName EXIF tag [0xa430]

#### --copyright
A string which will be inserted into the Copyright EXIF tag [0x8298]
