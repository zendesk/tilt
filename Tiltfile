# -*- mode: Python -*-

update_settings(max_parallel_updates=6)  # Run lots of tests at once

# Prerequisite: you need up-to-date node_modules to run these tests locally
# (won't be necessary for running tests on a container)
def yarn_install():
  local_resource("yarn_install", "cd web && yarn", deps=['web/package.json', 'web/yarn.lock'])

yarn_install()


# Approach 1: Tilt runs `yarn test` in the background, failures aren't immediately apparent
# and you need to check the logs to see if anything interesting has happened
local_resource("yarn_test_watch", serve_cmd="cd web && yarn run test --notify", resource_deps=["yarn_install"])

# Approach 2: a `test` item for every test file (thin wrapper around `local_resource`)
# that runs when the test file/associated files change.
# Note that this is a very naive way of telling which files should trigger foo.test.tsx
# (currently just prefix matching) -- there might be a more sophisticated way
web_src_files = [os.path.basename(f) for f in listdir('web/src')]
test_files = [f for f in web_src_files if f.endswith('test.ts') or f.endswith('test.tsx')]


def shortname(file):
  return file.replace('.test.tsx', '').replace('.test.ts', '')


for tf in test_files:
  short = shortname(tf)
  deps = [os.path.join('web/src/', f) for f in web_src_files if f.startswith(short)]
  test(short, "cd web && yarn test --watchAll=false %s" % tf, deps=deps, resource_deps=["yarn_install"])
