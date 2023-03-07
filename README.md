# pixe
[![flake8](https://github.com/ithuna/pixe/actions/workflows/flake8.yml/badge.svg)](https://github.com/ithuna/pixe/actions/workflows/flake8.yml) [![pytest](https://github.com/ithuna/pixe/actions/workflows/pytest.yml/badge.svg)](https://github.com/ithuna/pixe/actions/workflows/pytest.yml) [![Code style: black](https://img.shields.io/badge/code%20style-black-000000.svg)](https://github.com/psf/black)

A digital helper to keep your files neat and tidy.

In its most basic invocation: `pixe <directory with files>` `pixe` will copy all JPG files from a source directory into a new set of subdirectories based on capture date. These files will also be renamed based on said capture date and a calculated SHA1 hash.

### subdirectories

Subdirectories will be created, as needed, in the destination folder to match the creation date of the files being processed and will take the following form: `YYYY/M`

### renamed files
Each file (whether moved or copied) will have a new name applied to it based on the following pattern:
`YYYYMMDD_hhmmss_<SHA1SUM>.jpg`

The datetime info is taken from the image capture datetime. The SHA1SUM is calculated based on the image data only (does not include image metadata).

### example

Given a directory of images, `dirA` which contains a few image files:
```
dirA
├── IMG_0001.jpg
├── IMG_0002.jpg
└── IMG_1234.jpg
```

Running `pixe /path/to/dirA` from within a second directory, `dirB` would result in the following directory structure:

```
dirB
├── 2021
│   └── 12
│       └── 20211225_062223_7d97e98f8af710c7e7fe703abc8f639e0ee507c4.jpg
└── 2022
    ├── 2
    │   └── 20220202_123101_447d306060631570b7713ea48e74103c68eab0a3.jpg
    └── 3
        └── 20220316_232122_321c7d6f5be8739a8974e4c3512e3226eb6704a7.jpg
```

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
