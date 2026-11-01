import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().parents[1] / 'nix' / 'install-app.sh'

class InstallerTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.base = Path(self.tmp.name)
        tools = self.base / 'bin'
        tools.mkdir()
        nix = tools / 'nix'
        nix.write_text('''#!/usr/bin/env python3
import os, pathlib, sys
if os.environ.get('FAIL_BUILD'): sys.exit(1)
args=sys.argv[1:]
assert args[:4] == ['--extra-experimental-features','nix-command flakes','profile','add']
assert args[-1] == 'path:/test-source#ryotunes'
p=pathlib.Path(args[args.index('--profile')+1])/'bin'
p.mkdir(parents=True)
f=p/'ryotunes'; f.write_text('#!/bin/sh\\nexit 0\\n');f.chmod(0o755)
''')
        nix.chmod(0o755)
        self.env = dict(os.environ, PATH=str(tools)+os.pathsep+os.environ['PATH'],
                        XDG_STATE_HOME=str(self.base/'state'), XDG_DATA_HOME=str(self.base/'data'), XDG_BIN_HOME=str(self.base/'commands'),
                        RYOMARKET_APP_NAME='ryotunes', RYOMARKET_APP_TITLE='Ryotunes',
                        RYOMARKET_APP_PROGRAM='ryotunes', RYOMARKET_APP_FLAKE='path:/test-source',
                        RYOMARKET_APP_ICON='/test/icon.png')
        self.root = self.base/'state/ryomarket/flake-apps/ryotunes'
        self.desktop = self.base/'data/applications/dev.ryoku.ryomarket.ryotunes.desktop'

    def run_installer(self, *args, fail=False):
        env = dict(self.env)
        if fail: env['FAIL_BUILD']='1'
        return subprocess.run(['bash','-euo','pipefail',str(SCRIPT),*args],env=env,capture_output=True,text=True)

    def test_install_and_remove_preserve_other_files(self):
        unrelated = self.base/'data/unrelated'
        unrelated.parent.mkdir()
        unrelated.write_text('keep')
        result=self.run_installer()
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertTrue((self.root/'current/profile/bin/ryotunes').is_file())
        self.assertTrue(self.desktop.is_symlink())
        self.assertTrue((self.base/'commands/ryotunes').is_symlink())
        self.assertIn(str(self.root/'current/profile/bin/ryotunes'), self.desktop.read_text())
        self.assertEqual(self.run_installer('--remove').returncode,0)
        self.assertFalse(self.root.exists())
        self.assertFalse(self.desktop.is_symlink())
        self.assertFalse((self.base/'commands/ryotunes').is_symlink())
        self.assertEqual(unrelated.read_text(),'keep')

    def test_failed_update_keeps_working_generation(self):
        self.assertEqual(self.run_installer().returncode,0)
        previous=(self.root/'current').readlink()
        result=self.run_installer(fail=True)
        self.assertNotEqual(result.returncode,0)
        self.assertEqual((self.root/'current').readlink(),previous)
        self.assertTrue((self.root/'current/profile/bin/ryotunes').is_file())

    def test_foreign_desktop_is_not_overwritten(self):
        self.desktop.parent.mkdir(parents=True)
        self.desktop.write_text('foreign')
        self.assertNotEqual(self.run_installer().returncode,0)
        self.assertEqual(self.desktop.read_text(),'foreign')
        self.assertFalse(self.root.exists())

    def test_foreign_command_is_not_overwritten(self):
        command=self.base/'commands/ryotunes'
        command.parent.mkdir()
        command.write_text('keep')
        self.assertNotEqual(self.run_installer().returncode,0)
        self.assertEqual(command.read_text(),'keep')
        self.assertFalse((self.root/'current').exists())

    def test_unmanaged_directory_cannot_be_removed(self):
        self.root.mkdir(parents=True)
        (self.root/'personal').write_text('keep')
        self.assertNotEqual(self.run_installer('--remove').returncode,0)
        self.assertEqual((self.root/'personal').read_text(),'keep')

if __name__ == '__main__':
    unittest.main()
