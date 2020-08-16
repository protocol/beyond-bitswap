import subprocess

def prepareRun():
    res = subprocess.run(["ls"])
    print(res.stdout)

prepareRun()