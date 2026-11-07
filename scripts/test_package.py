import plistlib
from pathlib import Path
import unittest
import xml.etree.ElementTree as ET

from package import ROOT, release_version


class ReleaseTests(unittest.TestCase):
    def test_version_contract(self):
        self.assertEqual(release_version('v1.2.3'), '1.2.3')
        self.assertEqual(release_version('255.255.65535'), '255.255.65535')
        for tag in ['latest', 'v1.2', 'v1.2.3-beta', 'v256.0.0', 'v1.256.0', 'v1.0.65536', 'v01.2.3', 'v1.2.3;echo bad']:
            with self.subTest(tag=tag), self.assertRaises(ValueError):
                release_version(tag)

    def test_macos_bundle_contract(self):
        with (ROOT / 'packaging/macos/Info.plist').open('rb') as f:
            info = plistlib.load(f)
        self.assertEqual(info['CFBundleExecutable'], 'xiterm')
        self.assertEqual(info['CFBundleIconFile'], 'xiterm.icns')
        self.assertTrue((ROOT / 'assets/icons' / info['CFBundleIconFile']).is_file())

    def test_msi_upgrade_and_icon_contract(self):
        root = ET.parse(ROOT / 'packaging/windows/Package.wxs').getroot()
        ns = {'w': 'http://wixtoolset.org/schemas/v4/wxs'}
        package = root.find('w:Package', ns)
        self.assertEqual(package.get('UpgradeCode'), '588E216D-73A0-420C-8176-D045E3B87818')
        self.assertIsNotNone(package.find('w:MajorUpgrade', ns))
        self.assertEqual(package.find('w:Icon', ns).get('SourceFile'), '$(var.IconPath)')


if __name__ == '__main__':
    unittest.main()
