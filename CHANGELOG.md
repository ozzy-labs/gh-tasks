# Changelog

All notable changes to this project will be documented in this file. See [release-please-config.json](./release-please-config.json) for the automated release flow.

## [0.2.0](https://github.com/ozzy-labs/gh-tasks/compare/v0.1.1...v0.2.0) (2026-05-06)


### chore

* release-as v0.2.0 ([#341](https://github.com/ozzy-labs/gh-tasks/issues/341)) ([ac4bdfe](https://github.com/ozzy-labs/gh-tasks/commit/ac4bdfec3340adaaf0c533c58febc891bf5d959e))


### Features

* **cmd:** --namespace and --force for install-skills ([#335](https://github.com/ozzy-labs/gh-tasks/issues/335)) ([12b5b8c](https://github.com/ozzy-labs/gh-tasks/commit/12b5b8cc1c9632d6cb2d0355ef574d4659cd5d65))
* **cmd:** add install-skills command + claude-code adapter ([#329](https://github.com/ozzy-labs/gh-tasks/issues/329)) ([45c762c](https://github.com/ozzy-labs/gh-tasks/commit/45c762c35fa69481004e303e40e96905b27097ee))
* **cmd:** codex-cli adapter for install-skills ([#332](https://github.com/ozzy-labs/gh-tasks/issues/332)) ([36f27f5](https://github.com/ozzy-labs/gh-tasks/commit/36f27f5c2fda47cd9a66be0390e9ca0a6de20690))
* **cmd:** copilot adapter for install-skills ([#334](https://github.com/ozzy-labs/gh-tasks/issues/334)) ([aaab086](https://github.com/ozzy-labs/gh-tasks/commit/aaab086094c509348b8d049104733f8e993d4621))
* **cmd:** gemini-cli adapter for install-skills ([#333](https://github.com/ozzy-labs/gh-tasks/issues/333)) ([ad76905](https://github.com/ozzy-labs/gh-tasks/commit/ad7690539f55cfd6d972ec97c365231ace2d4471))
* **cmd:** uninstall flag for install-skills ([#336](https://github.com/ozzy-labs/gh-tasks/issues/336)) ([47e99fb](https://github.com/ozzy-labs/gh-tasks/commit/47e99fbd219de1526e8113ae04aa3c5352de586f))

## [0.1.1](https://github.com/ozzy-labs/gh-tasks/compare/v0.1.0...v0.1.1) (2026-05-06)


### Bug Fixes

* **ci:** pass --repo to gh release upload in checksums job ([#325](https://github.com/ozzy-labs/gh-tasks/issues/325)) ([570c331](https://github.com/ozzy-labs/gh-tasks/commit/570c33133942b9d96898c46bb23c68ee388a71cc))

## [0.1.0](https://github.com/ozzy-labs/gh-tasks/compare/v0.1.0...v0.1.0) (2026-05-06)


### ⚠ BREAKING CHANGES

* relocate renovate preset to configs/skills-sync ([#241](https://github.com/ozzy-labs/gh-tasks/issues/241))
* relocate projects v2 templates to templates/ ([#240](https://github.com/ozzy-labs/gh-tasks/issues/240))
* move skill source dir from src/skills to skills ([#239](https://github.com/ozzy-labs/gh-tasks/issues/239))
* unify on cobra flags, drop deps.argv legacy parser path ([#135](https://github.com/ozzy-labs/gh-tasks/issues/135)) (#222)
* **internal:** \`period.Of\`, \`period.SuggestMilestoneTitle\`, and \`period.FormatLocalISODate\` now take \`period.Options\` instead of separate positional args.
* phase 7 — go cutover, delete ts implementation ([#97](https://github.com/ozzy-labs/gh-tasks/issues/97)) (#114)

### refactor

* **internal:** switch period helpers to options struct ([#215](https://github.com/ozzy-labs/gh-tasks/issues/215)) ([03b41d8](https://github.com/ozzy-labs/gh-tasks/commit/03b41d8cdbe3ced7d02e531b37f888a705d657bc))
* move skill source dir from src/skills to skills ([#239](https://github.com/ozzy-labs/gh-tasks/issues/239)) ([f374fee](https://github.com/ozzy-labs/gh-tasks/commit/f374fee9a2b317addaef7ab74693880a8e7113fd))
* phase 7 — go cutover, delete ts implementation ([#97](https://github.com/ozzy-labs/gh-tasks/issues/97)) ([#114](https://github.com/ozzy-labs/gh-tasks/issues/114)) ([ae851a9](https://github.com/ozzy-labs/gh-tasks/commit/ae851a902e3c2c9c158574a838f7b9ff90cdbcba))
* relocate projects v2 templates to templates/ ([#240](https://github.com/ozzy-labs/gh-tasks/issues/240)) ([953ada9](https://github.com/ozzy-labs/gh-tasks/commit/953ada9636aa958b96c10130564c67e91c3814e6))
* relocate renovate preset to configs/skills-sync ([#241](https://github.com/ozzy-labs/gh-tasks/issues/241)) ([881a510](https://github.com/ozzy-labs/gh-tasks/commit/881a5107e2d56a9e3c94ff45a7eade938591d1a4))
* unify on cobra flags, drop deps.argv legacy parser path ([#135](https://github.com/ozzy-labs/gh-tasks/issues/135)) ([#222](https://github.com/ozzy-labs/gh-tasks/issues/222)) ([9dfd429](https://github.com/ozzy-labs/gh-tasks/commit/9dfd429ea21daf2e179b11eafb001aff2c73e622))


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
* **internal:** add i18n catalog reference completeness scanner ([#273](https://github.com/ozzy-labs/gh-tasks/issues/273)) ([da8278a](https://github.com/ozzy-labs/gh-tasks/commit/da8278ae2f5ca3b1f3b69c13223fb0e83e5c0a6d))
* **internal:** cursor pagination for graphql list queries ([#252](https://github.com/ozzy-labs/gh-tasks/issues/252)) ([ebfa109](https://github.com/ozzy-labs/gh-tasks/commit/ebfa109118dac4fe055a0697c0487a1238a5ce15)), closes [#244](https://github.com/ozzy-labs/gh-tasks/issues/244) [#251](https://github.com/ozzy-labs/gh-tasks/issues/251)
* publish skills-sync renovate preset for consumer auto-update ([#15](https://github.com/ozzy-labs/gh-tasks/issues/15)) ([e15589c](https://github.com/ozzy-labs/gh-tasks/commit/e15589c1c4844760fc18ea60884ea8e061e21548))
* **skills:** add v0.1.0 skill ssot stubs (task-* x6) ([#9](https://github.com/ozzy-labs/gh-tasks/issues/9)) ([40c8185](https://github.com/ozzy-labs/gh-tasks/commit/40c81855693d6ee0b0cf5aca92e28ad14ade45a4))
* **skills:** finalize adapter delivery via commons sync-skills.sh ([#68](https://github.com/ozzy-labs/gh-tasks/issues/68)) ([cbbc779](https://github.com/ozzy-labs/gh-tasks/commit/cbbc779a35bc37c282cd2b68f405c784896ec094)), closes [#16](https://github.com/ozzy-labs/gh-tasks/issues/16)
* **skills:** implement adapter layer for skills SSOT distribution ([#10](https://github.com/ozzy-labs/gh-tasks/issues/10)) ([38a7abd](https://github.com/ozzy-labs/gh-tasks/commit/38a7abddc40eec7683018f3191d5877c30bcddec))
* **templates:** provide projects v2 yaml templates under packages/templates/ ([#60](https://github.com/ozzy-labs/gh-tasks/issues/60)) ([ac1932b](https://github.com/ozzy-labs/gh-tasks/commit/ac1932b53c7716e4f55d0f9e364f5a8adacc0c51))


### Bug Fixes

* address critical and important review findings ([#243](https://github.com/ozzy-labs/gh-tasks/issues/243)) ([efd1850](https://github.com/ozzy-labs/gh-tasks/commit/efd185098d5a315ed58948a808bc303ccdcb1dcc))
* align implementation with design docs across 13 discrepancies ([#69](https://github.com/ozzy-labs/gh-tasks/issues/69)) ([22d6d37](https://github.com/ozzy-labs/gh-tasks/commit/22d6d3776ab01013162063bf2263e69d85d6583c))
* **ci:** correct release checksums asset pattern ([#299](https://github.com/ozzy-labs/gh-tasks/issues/299)) ([352cc5b](https://github.com/ozzy-labs/gh-tasks/commit/352cc5bbb9fea380442cee079b3b4a108c7b6222))
* **ci:** exclude .claude/worktrees from golangci-lint and i18n check ([#227](https://github.com/ozzy-labs/gh-tasks/issues/227)) ([f93d5b1](https://github.com/ozzy-labs/gh-tasks/commit/f93d5b1157d764f11050c94fc5dc8a0d443e14f9))
* **ci:** gate release-smoke on release workflow completion ([#276](https://github.com/ozzy-labs/gh-tasks/issues/276)) ([6d5c92e](https://github.com/ozzy-labs/gh-tasks/commit/6d5c92e402bda27dead1bf311a6dfc5ee04ea32b))
* **ci:** inject release version into binary via release-please extra-files ([#253](https://github.com/ozzy-labs/gh-tasks/issues/253)) ([7614e9c](https://github.com/ozzy-labs/gh-tasks/commit/7614e9c579bba33669e78231380f569b45a099c0))
* **ci:** pin workflow files in commons sync per commons adr-0012 ([#248](https://github.com/ozzy-labs/gh-tasks/issues/248)) ([ea09c59](https://github.com/ozzy-labs/gh-tasks/commit/ea09c59e096dd589edbf67102b159c16415b7491))
* **ci:** skip release-smoke gracefully when no release exists ([#292](https://github.com/ozzy-labs/gh-tasks/issues/292)) ([2ee9364](https://github.com/ozzy-labs/gh-tasks/commit/2ee9364cb3599fbd6dcc33cc155dcfd6a96e4483))
* **cmd:** align standup trailing newline with ts parity ([#190](https://github.com/ozzy-labs/gh-tasks/issues/190)) ([b075da4](https://github.com/ozzy-labs/gh-tasks/commit/b075da440d6c217ecb9809691c7a49487ba179b7))
* **cmd:** cache parsed iteration start in plan upcoming sort ([#208](https://github.com/ozzy-labs/gh-tasks/issues/208)) ([39b9547](https://github.com/ozzy-labs/gh-tasks/commit/39b9547fdd4338938b127b8966d6c769d124c991))
* **cmd:** extend wraptransport adoption to remaining handlers ([#303](https://github.com/ozzy-labs/gh-tasks/issues/303)) ([cc87839](https://github.com/ozzy-labs/gh-tasks/commit/cc8783986c35ae00d43888fb9af7c4b5c3622c5d))
* **cmd:** implement build-skills --check-diff for CI dogfooding ([#213](https://github.com/ozzy-labs/gh-tasks/issues/213)) ([80a972c](https://github.com/ozzy-labs/gh-tasks/commit/80a972c5672b49fe1d804667fa100a374fd4686e))
* **cmd:** sanitize --dist values in build-skills ([#200](https://github.com/ozzy-labs/gh-tasks/issues/200)) ([8efac76](https://github.com/ozzy-labs/gh-tasks/commit/8efac7669a23e3f98c6e333e9b79a083c7e218b5))
* **cmd:** warn on graphql partial errors in list cmd (poc for [#285](https://github.com/ozzy-labs/gh-tasks/issues/285) c-5) ([#297](https://github.com/ozzy-labs/gh-tasks/issues/297)) ([2747724](https://github.com/ozzy-labs/gh-tasks/commit/2747724b86b57672f2967452d5c6f22d19623685))
* drop leading slash from rest milestone create path ([#318](https://github.com/ozzy-labs/gh-tasks/issues/318)) ([7f84e3e](https://github.com/ozzy-labs/gh-tasks/commit/7f84e3e427689a2d89b6494e0f423d9ac4ba0463))
* fall back to git remote when --repo= is empty ([#118](https://github.com/ozzy-labs/gh-tasks/issues/118)) ([#180](https://github.com/ozzy-labs/gh-tasks/issues/180)) ([12e8e58](https://github.com/ozzy-labs/gh-tasks/commit/12e8e588492b2538c885f90a6deac5b9c86f689f))
* green up scaffold lint, typecheck, and tests ([#1](https://github.com/ozzy-labs/gh-tasks/issues/1)) ([592a260](https://github.com/ozzy-labs/gh-tasks/commit/592a26076f2612a109d8f6c33c326d9dcaecd54c))
* ignore dev binary in packages/gh-tasks/bin/ ([#78](https://github.com/ozzy-labs/gh-tasks/issues/78)) ([21141f5](https://github.com/ozzy-labs/gh-tasks/commit/21141f5ee6e5471b4ad527a5e223be487427bdb8))
* **internal:** isolate defaultGetRemoteURL from inherited git env vars ([#302](https://github.com/ozzy-labs/gh-tasks/issues/302)) ([8689f91](https://github.com/ozzy-labs/gh-tasks/commit/8689f91205794f38a63cb32e86de44309450b360))
* **internal:** nil-guard inner connections in 6 paginators ([#286](https://github.com/ozzy-labs/gh-tasks/issues/286)) ([98556a9](https://github.com/ozzy-labs/gh-tasks/commit/98556a96f16120c8aa6ba2c45e9e403864fe41f5))
* **internal:** panic on unrecognized period instead of returning empty range ([#185](https://github.com/ozzy-labs/gh-tasks/issues/185)) ([06845c7](https://github.com/ozzy-labs/gh-tasks/commit/06845c7c3047eeea618cb9a82cb9b974ab256489))
* resolve docs/implementation drift across 4 areas ([#71](https://github.com/ozzy-labs/gh-tasks/issues/71)) ([cff4f73](https://github.com/ozzy-labs/gh-tasks/commit/cff4f73cf77c92cc760acb7723d73e0ab423b6ac))
* surface per-key config errors for non-string toml scalars ([#119](https://github.com/ozzy-labs/gh-tasks/issues/119) [#124](https://github.com/ozzy-labs/gh-tasks/issues/124)) ([#182](https://github.com/ozzy-labs/gh-tasks/issues/182)) ([111079e](https://github.com/ozzy-labs/gh-tasks/commit/111079eb51b4b0fe274c22c12a8c0e630b844c81))
* surface silent errors and resolve locale on config error ([#116](https://github.com/ozzy-labs/gh-tasks/issues/116) [#117](https://github.com/ozzy-labs/gh-tasks/issues/117) [#132](https://github.com/ozzy-labs/gh-tasks/issues/132)) ([#181](https://github.com/ozzy-labs/gh-tasks/issues/181)) ([8b231c8](https://github.com/ozzy-labs/gh-tasks/commit/8b231c847b55408552af50167caa9c29905ac7f1))
* tighten i18ncheck decorative whitelist and expand tests ([#144](https://github.com/ozzy-labs/gh-tasks/issues/144) [#178](https://github.com/ozzy-labs/gh-tasks/issues/178) [#146](https://github.com/ozzy-labs/gh-tasks/issues/146)) ([#183](https://github.com/ozzy-labs/gh-tasks/issues/183)) ([cdda925](https://github.com/ozzy-labs/gh-tasks/commit/cdda9252f880c767b8219e088ec6c177f0c3d414))


### Performance

* **cmd:** cache default git remote lookup with sync.Once ([#220](https://github.com/ozzy-labs/gh-tasks/issues/220)) ([8028ecc](https://github.com/ozzy-labs/gh-tasks/commit/8028ecc095d1a462a3d9cdbd2103a307ed245cf5))

## [Unreleased]

- Initial scaffold.
