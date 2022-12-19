@echo off
git add *
git commit -m'auto'
git push
git tag v1.0.6
git push --tags
pause