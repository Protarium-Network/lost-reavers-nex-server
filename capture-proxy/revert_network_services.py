"""Restores Cemu's network_services.xml from the pre-capture backup."""
import shutil

SRC = r"C:\Users\evant\AppData\Roaming\Cemu\network_services_backup\network_services.xml.bak"
DST = r"C:\Users\evant\AppData\Roaming\Cemu\network_services.xml"

shutil.copyfile(SRC, DST)
print(f"Restored {DST} from backup.")
