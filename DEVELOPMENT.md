# Project Development

## Deployment

1. finish changes and update the `pyproject.toml` file to reflect the version
2. tag the latest revision and push the tags to GitHub
3. Ensure your build env is setup (https://packaging.python.org/en/latest/tutorials/packaging-projects/)
4. Upgrade the build env `python3 -m pip install --upgrade build`
5. Build the latest package version `python3 -m build`
6. Upload the newly built version to the test repo `python3 -m twine upload --repository testpypi dist/*`
7. Check the build on https://test.pypi.org
8. Upload the build to the main pypi repo `python3 -m twine upload dist/*`