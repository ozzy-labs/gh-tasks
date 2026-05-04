# Changelog

## 0.1.0 (2026-05-04)


### Features

* **cli:** add `gh tasks projects init` to bootstrap project from yaml ([#70](https://github.com/ozzy-labs/gh-tasks/issues/70)) ([2aa4dff](https://github.com/ozzy-labs/gh-tasks/commit/2aa4dff5a717e67e199b8726ff4a72aa3ef8b0c0))
* **cli:** implement gh tasks add for org/user scope ([#50](https://github.com/ozzy-labs/gh-tasks/issues/50)) ([826b1ef](https://github.com/ozzy-labs/gh-tasks/commit/826b1ef21c18d5a72046f400ddd6a81a5f50af9c))
* **cli:** implement gh tasks done for org/user scope ([#53](https://github.com/ozzy-labs/gh-tasks/issues/53)) ([ba39c06](https://github.com/ozzy-labs/gh-tasks/commit/ba39c066c70fb3150bb1ed6dcc8386ec2b78f267))
* **cli:** implement gh tasks done for repo scope ([#19](https://github.com/ozzy-labs/gh-tasks/issues/19)) ([2645f9b](https://github.com/ozzy-labs/gh-tasks/commit/2645f9b750d6d79d12d3986509de71f92b39a2ff))
* **cli:** implement gh tasks link for org/user scope ([#54](https://github.com/ozzy-labs/gh-tasks/issues/54)) ([154d6c7](https://github.com/ozzy-labs/gh-tasks/commit/154d6c7fe181c2951b1df47e2c45730bac10e368))
* **cli:** implement gh tasks link for repo scope ([#25](https://github.com/ozzy-labs/gh-tasks/issues/25)) ([7a12c4b](https://github.com/ozzy-labs/gh-tasks/commit/7a12c4b5926ad76e7b0a7f355e8ac69505879a7e)), closes [#20](https://github.com/ozzy-labs/gh-tasks/issues/20)
* **cli:** implement gh tasks list for org/user scope ([#51](https://github.com/ozzy-labs/gh-tasks/issues/51)) ([5998d42](https://github.com/ozzy-labs/gh-tasks/commit/5998d42a7a3a508151748d3c1d7c9e7907fe22da))
* **cli:** implement gh tasks list for repo scope ([#17](https://github.com/ozzy-labs/gh-tasks/issues/17)) ([8331a47](https://github.com/ozzy-labs/gh-tasks/commit/8331a47048458247ef377f78b460855710670d3e))
* **cli:** implement gh tasks plan (dry-run) for repo scope ([#27](https://github.com/ozzy-labs/gh-tasks/issues/27)) ([476f77b](https://github.com/ozzy-labs/gh-tasks/commit/476f77bf094b05d9afc5917b78a8d615722b878e)), closes [#22](https://github.com/ozzy-labs/gh-tasks/issues/22)
* **cli:** implement gh tasks plan for org/user scope ([#56](https://github.com/ozzy-labs/gh-tasks/issues/56)) ([1e3834a](https://github.com/ozzy-labs/gh-tasks/commit/1e3834a839a7f563e6f097e59e6070e5bfd84b5c))
* **cli:** implement gh tasks plan write mode (milestone create + bind) ([#31](https://github.com/ozzy-labs/gh-tasks/issues/31)) ([3e62298](https://github.com/ozzy-labs/gh-tasks/commit/3e622981d16e09e8f2ca233f249c190d69b97c5a))
* **cli:** implement gh tasks review for org/user scope ([#57](https://github.com/ozzy-labs/gh-tasks/issues/57)) ([4cd509b](https://github.com/ozzy-labs/gh-tasks/commit/4cd509b897b76127fe394a2a84c90013a0174da3))
* **cli:** implement gh tasks review for repo scope ([#28](https://github.com/ozzy-labs/gh-tasks/issues/28)) ([267be86](https://github.com/ozzy-labs/gh-tasks/commit/267be86e73d7463522d0523d52270f51352d3cb3)), closes [#23](https://github.com/ozzy-labs/gh-tasks/issues/23)
* **cli:** implement gh tasks standup --mine author/assignee filtering ([#47](https://github.com/ozzy-labs/gh-tasks/issues/47)) ([33c2bd8](https://github.com/ozzy-labs/gh-tasks/commit/33c2bd813ea21731e375745a7b250a1b8652eb73))
* **cli:** implement gh tasks standup for org/user scope ([#58](https://github.com/ozzy-labs/gh-tasks/issues/58)) ([f7aafc1](https://github.com/ozzy-labs/gh-tasks/commit/f7aafc16467ab4f73a62e707043fa740641acb1e)), closes [#43](https://github.com/ozzy-labs/gh-tasks/issues/43)
* **cli:** implement gh tasks standup for repo scope ([#29](https://github.com/ozzy-labs/gh-tasks/issues/29)) ([8ed3f6a](https://github.com/ozzy-labs/gh-tasks/commit/8ed3f6ab1bab33db9245a41b14ed25dc70d769c5)), closes [#24](https://github.com/ozzy-labs/gh-tasks/issues/24)
* **cli:** implement gh tasks today for org/user scope ([#52](https://github.com/ozzy-labs/gh-tasks/issues/52)) ([a8be056](https://github.com/ozzy-labs/gh-tasks/commit/a8be0564c64e2a037be7e07b6ff32e829f05b2ba))
* **cli:** implement gh tasks today for repo scope ([#18](https://github.com/ozzy-labs/gh-tasks/issues/18)) ([dd1e1af](https://github.com/ozzy-labs/gh-tasks/commit/dd1e1af71b56067942e3b7ad1411117ae755d432))
* **cli:** implement gh tasks triage for org/user scope ([#55](https://github.com/ozzy-labs/gh-tasks/issues/55)) ([745d659](https://github.com/ozzy-labs/gh-tasks/commit/745d659be030f1ce4870ad19aea5a3c8c86f9afb)), closes [#40](https://github.com/ozzy-labs/gh-tasks/issues/40)
* **cli:** implement gh tasks triage for repo scope ([#26](https://github.com/ozzy-labs/gh-tasks/issues/26)) ([929a0f0](https://github.com/ozzy-labs/gh-tasks/commit/929a0f0d42bc73f7979ca67033877997da9dae8f)), closes [#21](https://github.com/ozzy-labs/gh-tasks/issues/21)
* **cli:** improve locale resolution with POSIX semantics and tests ([#14](https://github.com/ozzy-labs/gh-tasks/issues/14)) ([36e527c](https://github.com/ozzy-labs/gh-tasks/commit/36e527c6c4da0717e587bce44f4d9a5d5b01d97c))
* **cli:** respect local timezone for weekly period boundary ([#61](https://github.com/ozzy-labs/gh-tasks/issues/61)) ([c3ba2f8](https://github.com/ozzy-labs/gh-tasks/commit/c3ba2f8d0c58ba810d6e360312be79f99a9425bd))
* **cli:** scaffold lib foundation (scope detection, octokit client, queries) ([#11](https://github.com/ozzy-labs/gh-tasks/issues/11)) ([cdf5bbe](https://github.com/ozzy-labs/gh-tasks/commit/cdf5bbea4d4d07d91ddadeda098df6f73a36f5d0))
* **cli:** scaffold org/user scope foundation (project v2 client + queries) ([#48](https://github.com/ozzy-labs/gh-tasks/issues/48)) ([42900d0](https://github.com/ozzy-labs/gh-tasks/commit/42900d09c400c9d369fcff8856d1b077ddc1d439))
* **cli:** support ~/.config/ozzylabs/gh-tasks.toml for lang and default_scope ([#46](https://github.com/ozzy-labs/gh-tasks/issues/46)) ([ae240b8](https://github.com/ozzy-labs/gh-tasks/commit/ae240b88027faec97071fb7672be79a07fa60d04))


### Bug Fixes

* align implementation with design docs across 13 discrepancies ([#69](https://github.com/ozzy-labs/gh-tasks/issues/69)) ([22d6d37](https://github.com/ozzy-labs/gh-tasks/commit/22d6d3776ab01013162063bf2263e69d85d6583c))
* green up scaffold lint, typecheck, and tests ([#1](https://github.com/ozzy-labs/gh-tasks/issues/1)) ([592a260](https://github.com/ozzy-labs/gh-tasks/commit/592a26076f2612a109d8f6c33c326d9dcaecd54c))
