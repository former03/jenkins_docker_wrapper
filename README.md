jenkins_docker_wrapper
=====================

Go implementation of a wrapper that allows to launch Docker containers without full rights in Docker from Jenkins job environment.


Implemented features:
--------------

- Launch docker containers via docker remote API

(Planned) features:
--------------

- Limit allowed images via regex from config file
- Limit allowed volume paths via regex from config file
- Read Docker image name from projekt.conf within the git root


Config file
------------


Per project config file projekt.conf
------------


Usage
-----

CLI Arguments
-------


Author
------

- [simonswine][simonswine] for [FORMER 03][former03]

[simonswine]: https://github.com/simonswine
[former03]: http://www.former03.de

License
-------

The MIT License (MIT)
