from setuptools import setup, find_packages

setup(
    name="pixe",
    version="0.5.0",
    py_modules=["pixe"],
    url="https://github.com/ithuna/pixe",
    license="Apache-2",
    author="Chris Wells",
    author_email="chris@ithuna.com",
    description="A digital helper to keep your files neat and tidy",
    include_package_data=True,
    install_requires=[
        "Click>=8.1.3,<8.2",
        "Pillow>=9.4.0,<9.5",
    ],
    entry_points={
        "console_scripts": [
            "pixe = pixe:cli",
        ],
    },
)
