from setuptools import setup, find_packages

setup(
    name="tomte",
    version="0.1.0",
    py_modules=["tomte"],
    url="https://github.com/fosforics/tomte",
    license="Apache-2",
    author="Chris Wells",
    author_email="chris@fosforics.com",
    description="A digital house gnome to keep your files neat and tidy",
    include_package_data=True,
    install_requires=[
        "Click>=8.1.3,<8.2",
        "Pillow>=9.4.0,<9.5",
    ],
    entry_points={
        "console_scripts": [
            "tomte = tomte:cli",
        ],
    },
)
