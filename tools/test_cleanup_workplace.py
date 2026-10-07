"""清理工具的拒收條件與保留副本，使用容器內的暫存目錄。"""
import importlib.util
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('cleanup_workplace', Path(__file__).with_name('cleanup-workplace.py'))
cleanup = importlib.util.module_from_spec(spec)
spec.loader.exec_module(cleanup)


class CleanupSafety(unittest.TestCase):
    def setUp(self):
        self.scratch = tempfile.TemporaryDirectory(prefix='san1-cleanup-')
        self.addCleanup(self.scratch.cleanup)
        self.root = Path(self.scratch.name)
        (self.root/'workplace').mkdir()

    def pair(self, temporary='temporary', retained='retained'):
        for name in (temporary, retained):
            p = self.root/'workplace'/name
            (p/'nested').mkdir(parents=True)
            (p/'nested/art.png').write_bytes(bytes(range(256)))
            (p/'receipt.json').write_text('{"source":"preserved"}\n')
        return f'workplace/{temporary}=workplace/{retained}'

    def run_cleanup(self, pairs, apply=True, report='workplace/result.json'):
        return cleanup.run(self.root, pairs, report, apply)

    def test_default_dry_run_keeps_both_directories(self):
        result = self.run_cleanup([self.pair()], False)
        self.assertTrue(result['passed'])
        self.assertEqual(result['deleted'], [])
        self.assertTrue((self.root/'workplace/temporary/nested/art.png').is_file())
        self.assertTrue((self.root/'workplace/retained/nested/art.png').is_file())

    def test_apply_removes_only_duplicates_and_preserves_nested_bytes(self):
        pairs = [self.pair(), self.pair('second', 'second-retained')]
        result = self.run_cleanup(pairs)
        self.assertEqual(result['deleted'], ['workplace/temporary','workplace/second'])
        for name in ('retained','second-retained'):
            self.assertEqual((self.root/'workplace'/name/'nested/art.png').read_bytes(), bytes(range(256)))
        self.assertFalse((self.root/'workplace/temporary').exists())
        self.assertFalse((self.root/'workplace/second').exists())

    def test_one_different_copy_rejects_whole_batch_before_deletion(self):
        pairs = [self.pair(), self.pair('second','second-retained')]
        (self.root/'workplace/second-retained/nested/art.png').write_bytes(b'changed')
        with self.assertRaisesRegex(ValueError, '不完全相同'):
            self.run_cleanup(pairs)
        self.assertTrue((self.root/'workplace/temporary/nested/art.png').is_file())
        self.assertTrue((self.root/'workplace/second/nested/art.png').is_file())

    def test_extra_file_rejects_deletion(self):
        pair = self.pair()
        (self.root/'workplace/temporary/unique.txt').write_text('unique')
        with self.assertRaises(ValueError):
            self.run_cleanup([pair])
        self.assertTrue((self.root/'workplace/temporary/unique.txt').is_file())

    def test_root_outside_absolute_and_parent_paths_are_rejected(self):
        self.pair()
        for n, bad in enumerate(('workplace','org_game/data','/tmp/data','workplace/../org_game')):
            with self.subTest(path=bad), self.assertRaises(ValueError):
                self.run_cleanup([bad+'=workplace/retained'], report=f'workplace/reject-{n}.json')
        self.assertTrue((self.root/'workplace/temporary').is_dir())

    def test_symlink_directory_cannot_escape_workplace(self):
        self.pair()
        outside = self.root/'outside'
        outside.mkdir()
        (outside/'unique').write_text('keep')
        (self.root/'workplace/link').symlink_to(outside, target_is_directory=True)
        with self.assertRaisesRegex(ValueError, '符號連結'):
            self.run_cleanup(['workplace/link=workplace/retained'])
        self.assertEqual((outside/'unique').read_text(), 'keep')

    def test_nested_symlink_is_rejected(self):
        pair = self.pair()
        (self.root/'workplace/temporary/link').symlink_to(self.root/'workplace/retained')
        with self.assertRaisesRegex(ValueError, '特殊檔'):
            self.run_cleanup([pair])
        self.assertTrue((self.root/'workplace/temporary/nested/art.png').is_file())

    def test_retained_directory_cannot_be_deleted_by_another_pair(self):
        pair = self.pair()
        self.pair('another','another-retained')
        with self.assertRaisesRegex(ValueError, '保留目錄'):
            self.run_cleanup([pair, 'workplace/retained=workplace/another-retained'])
        self.assertTrue((self.root/'workplace/temporary').is_dir())
        self.assertTrue((self.root/'workplace/retained').is_dir())

    def test_existing_receipt_is_never_overwritten(self):
        pair = self.pair()
        p = self.root/'workplace/result.json'
        p.write_text('existing')
        with self.assertRaises(ValueError):
            self.run_cleanup([pair])
        self.assertEqual(p.read_text(), 'existing')
        self.assertTrue((self.root/'workplace/temporary').is_dir())

    def test_receipt_inside_target_cannot_allow_deletion(self):
        pair = self.pair()
        with self.assertRaises(ValueError):
            self.run_cleanup([pair], report='workplace/temporary/result.json')
        self.assertTrue((self.root/'workplace/temporary/nested/art.png').is_file())


if __name__ == '__main__':
    unittest.main()
