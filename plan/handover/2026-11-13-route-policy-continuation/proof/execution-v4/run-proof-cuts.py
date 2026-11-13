#!/usr/bin/env python3
"""Main-thread-only, serialized RED/guarded restore/GREEN for the 27 proposals.

Prepared but never executed by its author. Read run-proof-cuts.md first.
All project execution goes through le job and the existing native test actions.
"""
import argparse
import datetime
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
import shutil
import signal
import subprocess
import sys
import tempfile

ROOT = Path('/home/thomas/Code/github.com/ze-software/ze/main')
PLAN = Path(__file__).resolve().parent
KERNEL = Path('/home/thomas/.cache/ze/runtime-kernel/7.3-runtime-amd64-runtime-docker-6727e257-c79de00c/vmlinuz')
UNIT27 = ['TestLocRIBForwardingMembersNonwinnerExplicitGroup',
          'TestLocRIBNonwinnerExplicitSecondaryChangesNotify',
          'TestLocRIBExplicitMembersRespectAdmissionRankAndDuplicates',
          'TestLocRIBUnchangedEffectivePathKeepsLatestProducerValues',
          'TestRouteViewNonwinnerExplicitGroupBothReaders']
UNIT25 = ['TestFIBKernel' + suffix for suffix in (
    'ReplayReassertsUnchangedForwarding', 'ReplayChangedMetadataUsesFullReplacement',
    'ReplayUncertainStateUsesFullProgramming', 'LiveChangesNeverUseReplayReassertion',
    'ReplayUnhandledUsesFullReplacement', 'ReplayUnhandledErrorDoesNotReplace',
    'ReplayReplaceKeepsFullProgramming', 'ReplayFailedReplacementNeedsFullRepair',
    'ReplayRefusalPreservesSuccessfulForwarding', 'ReplayWithoutOptionalBackendStillRepairs')]
UNIT26 = 'TestSysribExecutablePrivilegedCancellationJoinsRootChild'

# Every RED expression names the source-owned assertion reached by this cut.
# Matching ZE-OBSERVER-FAIL at that assertion establishes the earlier successful
# barriers by the fixture's sequential return-on-error flow. The retained row
# also records the exact earlier controls from cuts.current.json for inspection.
RED = {
  1: r'show rib lists 192\.0\.2\.0/24 from .*but the rib import chain rejects it',
  2: r'show rib candidates prefix 198\.18\.221\.0/24: connected path metadata did not match',
  3: r'192\.0\.2\.0/24: system RIB winner is bgp, want static',
  4: r'(198\.19\.21\.0/24: system RIB winner is static, want bgp|show rib candidates prefix 198\.19\.21\.0/24: static path metadata did not match distance=5)',
  5: r'the bgp path is hidden by .*want central',
  6: r'show rib candidates prefix 198\.18\.232\.0/24: static path metadata did not match',
  7: r'show rib candidates prefix 198\.(18\.232|19\.23)\.0/24: static path metadata did not match',
  8: r'(FIB 198\.18\.244\.0/24 replay-after=0:|route-metrics 10\.242\.1\.5: got)',
  9: r'FIB 198\.18\.244\.0/24 replay-after=0:.*want .*Action:withdraw',
 10: r'FIB 198\.18\.251\.0/24 replay-after=0:.*want .*Labels:\[16002\]',
 11: r'FIB 198\.18\.252\.0/24 replay-after=0:.*want .*SRv6SID:2001:db8:249::b',
 12: r'FIB (198\.18\.(244|249)\.0/24|10\.253\.[0-9]+\.[0-9]+/32) replay-after=[1-9][0-9]*:',
 13: r'bounded-feed reset did not fence outstanding output while its reader was stopped:',
 14: r'(owner replacement never withdrew both old-generation outputs|kernel 198\.19\.11\.0/24: want 0 routes)',
 15: r'disconnected static producer retained 198\.19\.21\.0/24 candidate:',
 16: r'authoritative metric 10\.249\.1\.1: got',
 17: r'authoritative metric 10\.249\.3\.1: got',
 18: r'lost zero-cost reachability must invalidate cached winner:',
 19: r'lost zero-cost reachability must invalidate cached winner:',
 20: r'routing-view ready gauge did not become one:',
 21: r'same-process routing recovery did not count reconstruction:',
 22: r'failed routing owner doctor omitted exact recovery command:',
 23: r'kernel 198\.18\.254\.0/24: want 0 routes',
 24: r'198\.18\.211\.0/24: want exactly one blackhole kernel entry proto 250,',
}

def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def require(condition, message):
    if not condition:
        raise RuntimeError(message)

def save(path, value):
    path.write_text(json.dumps(value, indent=2) + '\n')

def require_hash(path, expected):
    require(digest(path) == expected, f'Hash mismatch: {path}; refusing stale mutation/restoration')

def proposed_digest(source, patch):
    lines = patch.read_text().splitlines(keepends=True)
    headers = [index for index, line in enumerate(lines) if line.startswith('@@ ')]
    require(len(headers) == 1, 'Re-read a proposal that no longer has exactly one hunk')
    body = lines[headers[0] + 1:]
    before = ''.join(line[1:] for line in body if line.startswith((' ', '-')))
    after = ''.join(line[1:] for line in body if line.startswith((' ', '+')))
    current = source.read_text()
    require(current.count(before) == 1, 'Proposal old hunk is not unique/current')
    return hashlib.sha256(current.replace(before, after, 1).encode()).hexdigest()

def expect(pattern, text, why):
    require(re.search(pattern, text, re.M) is not None, why + ': ' + pattern)

class Executor:
    def __init__(self, session, output):
        self.le = ['./le', '--name', session]
        self.output = output
        self.unjoined = False
        self.events = []

    def job(self, label, argv, env=None):
        """One joined admitted command, retaining exact argv and quiet log."""
        command = self.le + ['job', 'run', 'label', label, 'quiet', 'command'] + argv
        print('RUN ' + label, flush=True)
        process = subprocess.Popen(command, cwd=ROOT, env=env, stdout=subprocess.PIPE,
                                   stderr=subprocess.PIPE, text=True)
        try:
            stdout, stderr = process.communicate()
        except BaseException:
            if process.poll() is None:
                process.send_signal(signal.SIGINT)
            try:
                stdout, stderr = process.communicate(timeout=180)
            except subprocess.TimeoutExpired:
                self.unjoined = True
                raise RuntimeError('Job did not join cancellation; source is retained for main to reconcile')
            raise
        (self.output / (label + '.stdout')).write_text(stdout)
        (self.output / (label + '.stderr')).write_text(stderr)
        found = re.search(r'^job [^\n]+, log (\S+)$', stdout, re.M)
        require(found is not None, f'{label}: no native quiet report/log; setup result is invalid')
        path = ROOT / found[1]
        require(path.is_file(), f'{label}: native quiet log missing: {path}')
        text = path.read_text()
        self.events.append({'label': label, 'argv': command, 'exit': process.returncode,
                            'quiet_log': str(path), 'quiet_log_sha256': digest(path)})
        save(self.output / 'jobs.json', self.events)
        return process.returncode, text

    def green_job(self, label, argv, env=None):
        code, text = self.job(label, argv, env)
        require(code == 0, f'{label}: build/setup/native command failed; see retained log')
        return text

    def native(self, label, guest, mode):
        text = self.green_job(label, self.le + ['test', 'qemu', 'docker-lab', 'kernel',
                              str(KERNEL), mode, 'timeout', '3000s', 'command', guest],
                              dict(os.environ, CGO_ENABLED='0'))
        for pattern in (r'^check-exit\s+0$', r'^lab-exit\s+0$',
                        r'^command-exit\s+0$', r'^interrupted\s+false$'):
            expect(pattern, text, label + ': native setup, timeout, or interruption invalidates proof')
        require(str(KERNEL) in text, label + ': native report does not name requested own kernel')
        return text

    def build_guest(self, label):
        text = self.green_job(label, self.le + ['test', 'qemu', 'guest-binaries'],
                              dict(os.environ, CGO_ENABLED='0'))
        environment = re.search(r'^environment\s+(.+)$', text, re.M)
        require(environment is not None, 'Guest builder gave no current binary environment')
        assignments = shlex.split(environment[1])
        require(len(assignments) == 3, 'Unexpected guest binary environment shape')
        values = dict(part.split('=', 1) for part in assignments)
        require(set(values) == {'ZE_BIN', 'ZE_STRIPPED_BIN', 'ZE_STRIPPED_FLOW_BIN'},
                'Guest builder changed environment contract; re-read its producer')
        binaries = {name: {'path': value, 'sha256': digest(ROOT / value)}
                    for name, value in values.items()}
        save(self.output / (label + '.binaries.json'), binaries)
        # GuestLeRel is adjacent to GuestBinariesRel; native harness builds it.
        guest_le = str(Path(values['ZE_BIN']).parent.parent / 'le')
        return assignments, guest_le

    def daemon_stage(self, row, attempt, stage):
        number = int(row['cut'][:2])
        assignments, guest_le = self.build_guest(f"rp6-cut-{row['cut']}-{stage}-build")
        directory = attempt / stage
        directory.mkdir()
        script = ['set -u']
        for carrier in row['primary_carriers']:
            stem = Path(carrier).stem
            target = directory / stem
            relative = str(target.relative_to(ROOT))
            exit_path = relative + '.exit'
            argv = ['env'] + assignments + [guest_le, 'test', 'qemu', 'all', 'test', carrier,
                                             'save', relative]
            begin = 'RP6_CARRIER_BEGIN ' + row['cut'] + ' ' + stage + ' ' + carrier
            end = 'RP6_CARRIER_END ' + row['cut'] + ' ' + stage + ' ' + carrier
            script.extend([f"printf '%s\\n' {shlex.quote(begin)}",
                f"if {shlex.join(argv)}; then",
                f"  printf '0\\n' > {shlex.quote(exit_path)}", 'else',
                f"  printf '%s\\n' \"$?\" > {shlex.quote(exit_path)}", 'fi',
                f"printf '%s\\n' {shlex.quote(end)}"])
        script.append('exit 0')
        guest = '\n'.join(script)
        (directory / 'guest-command.sh').write_text(guest + '\n')
        batch_log = self.native(f"rp6-cut-{row['cut']}-{stage}-daemon-batch", guest, 'harness')
        errors = []
        for carrier in row['primary_carriers']:
            stem = Path(carrier).stem
            target = directory / stem
            try:
                begin = 'RP6_CARRIER_BEGIN ' + row['cut'] + ' ' + stage + ' ' + carrier
                end = 'RP6_CARRIER_END ' + row['cut'] + ' ' + stage + ' ' + carrier
                segments = re.findall('^' + re.escape(begin) + r'\n(.*?)^' + re.escape(end) + '$',
                                      batch_log, re.M | re.S)
                require(len(segments) == 1, f'{carrier}: missing or repeated native output boundary')
                runner = segments[0]
                target.with_suffix('.runner.log').write_text(runner)
                code = int(target.with_suffix('.exit').read_text().strip())
                captures = list(target.rglob('client-stderr.log'))
                require(len(captures) == 1, f'{carrier}: expected exactly one complete client capture')
                captured = captures[0].read_text()
                require(f'selection: only {carrier} ran' in runner, f'{carrier}: wrong test selected')
                require('phases: 1 ran, 0 skipped, 1 planned' in runner, f'{carrier}: skipped/unrun phase')
                require('tests: 1 executed across the functional suites' in runner, f'{carrier}: no exact executed test count')
                expect(r'OK: sysrib placement (internal|forked), running owner and child count [01]',
                       captured, f'{carrier}: actual placement control not reached')
                require('panic:' not in captured and 'fatal error:' not in captured, f'{carrier}: runtime crash is not discrimination')
                if stage == 'red':
                    require(code != 0, f'{carrier}: mutation did not redden carrier')
                    expect(r'\b1/1\s+FAIL\s+[0-9]+\s+' + re.escape(stem) + r'\s*$', runner,
                           f'{carrier}: named test itself did not fail')
                    failures = [line.split('ZE-OBSERVER-FAIL:', 1)[1] for line in captured.splitlines()
                                if 'ZE-OBSERVER-FAIL:' in line]
                    require(failures, f'{carrier}: no source-owned observer assertion; setup/startup failure is invalid')
                    for failure in failures:
                        expect(RED[number], failure, f'{carrier}: wrong semantic RED for {row["cut"]}')
                else:
                    require(code == 0 and 'ALL PHASES PASSED' in runner, f'{carrier}: restored carrier failed')
                    expect(r'\b1/1\s+PASS\s+[0-9]+\s+' + re.escape(stem) + r'\s*$', runner,
                           f'{carrier}: named restored test did not pass')
                    require('ZE-OBSERVER-FAIL:' not in captured, f'{carrier}: restored observer failed')
                    for line in (ROOT / carrier).read_text().splitlines():
                        marker = 'expect=stderr:contains=OK: '
                        if line.startswith(marker):
                            require(line.split('contains=', 1)[1] in captured,
                                    f'{carrier}: missing full-success/cleanup marker {line}')
                save(directory / (stem + '.validated.json'), {
                    'carrier': carrier, 'stage': stage, 'actual_exit': code,
                    'capture': str(captures[0]), 'capture_sha256': digest(captures[0]),
                    'expected_semantic_red': row['expected_semantic_red'],
                    'earlier_controls_source_order': row['control_before_expected_red'],
                    'automated_checks_passed': True, 'human_full_capture_review_owed': True})
            except Exception as error:
                errors.append(f'{carrier}: {error}')
        require(not errors, '\n'.join(errors))

    def unit_stage(self, row, attempt, stage, tags):
        number = int(row['cut'][:2])
        names = UNIT25 if number == 25 else UNIT27
        packages = ['./internal/plugins/fib/kernel'] if number == 25 else [
            './internal/core/rib/locrib', './internal/component/sysrib']
        code, text = self.job(f"rp6-cut-{row['cut']}-{stage}-units",
            ['go', 'test', '-race', '-p', '1', '-v', '-count=1', '-tags', tags,
             '-run', '^(' + '|'.join(names) + ')$'] + packages,
             dict(os.environ, CGO_ENABLED='1'))
        target = 'TestFIBKernelReplayUnhandledErrorDoesNotReplace' if number == 25 else UNIT27[3]
        for name in names:
            expect(r'^=== RUN   ' + re.escape(name) + '$', text, 'Exact selected unit did not execute')
            status = 'FAIL' if stage == 'red' and name == target else 'PASS'
            expect(r'^--- ' + status + ': ' + re.escape(name) + r' \(', text, 'Wrong unit result')
        require('--- SKIP:' not in text and '[build failed]' not in text and '[setup failed]' not in text,
                'Unit skip/build/setup failure is invalid')
        require((code != 0) == (stage == 'red'), 'Unexpected selected unit exit')
        if stage == 'red':
            expected = 'backend error fell through to 1 full replacements' if number == 25 else 'equal effective offer lost its producer originals:'
            require(expected in text, 'Named target failed at a different assertion')
            if number == 25:
                require('unhandled backend error did not record pending failure' in text, 'Replay false/error did not expose pending-failure loss')
        if number == 27:
            for mode in ('local-authority', 'selected-json'):
                expect(r'^\s+--- PASS: TestRouteViewNonwinnerExplicitGroupBothReaders/' + mode + r' \(',
                       text, 'Both reader controls must stay healthy')

    def privileged_stage(self, row, attempt, stage, tags):
        executable = attempt / ('fixture-' + stage + '.test')
        self.green_job(f'rp6-cut-26-{stage}-build', ['env', 'GOOS=linux', 'CGO_ENABLED=0',
            'go', 'test', '-c', '-race=false', '-p', '1', '-tags', tags + ',integration',
            '-o', str(executable), './internal/test/fixture'], dict(os.environ, CGO_ENABLED='0'))
        save(attempt / (stage + '-elf.json'), {'path': str(executable), 'sha256': digest(executable)})
        # The root native command shares the host absolute checkout. The
        # private executable mount preserves CAP_KILL for a non-root test owner.
        guest = '\n'.join(['set -e', 'mkdir -p /tmp/rp6-control-exec',
            'mount -t tmpfs -o mode=0755,nodev tmpfs /tmp/rp6-control-exec',
            f'cp {shlex.quote(str(executable))} /tmp/rp6-control-exec/fixture.test',
            'chmod 0755 /tmp/rp6-control-exec/fixture.test',
            'setcap cap_kill+ep /tmp/rp6-control-exec/fixture.test',
            'getcap /tmp/rp6-control-exec/fixture.test',
            'cd /tmp/rp6-control-exec',
            'if sudo -n -u ze env HOME=/tmp TMPDIR=/tmp /tmp/rp6-control-exec/fixture.test '
            f'-test.v -test.count=1 -test.timeout=1m -test.run {shlex.quote("^" + UNIT26 + "$")}; then',
            "  printf 'UNIT26_ACTUAL_EXIT=0\\n'", 'else',
            "  printf 'UNIT26_ACTUAL_EXIT=%s\\n' \"$?\"", 'fi', 'exit 0'])
        text = self.native(f'rp6-cut-26-{stage}-native', guest, 'root')
        require('cap_kill=ep' in text, 'Native non-root test lacks actual CAP_KILL')
        expect(r'^=== RUN   ' + UNIT26 + '$', text, 'Privileged target did not execute')
        status = 'FAIL' if stage == 'red' else 'PASS'
        expect(r'^--- ' + status + ': ' + UNIT26 + r' \(', text, 'Wrong privileged target result')
        require('--- SKIP:' not in text and '[setup failed]' not in text, 'Root/permission/setup skip is invalid')
        if stage == 'red':
            require('actual inspection runner did not join its stopped root child on cancellation' in text,
                    'Privileged target failed before its authenticated stopped-root barrier')
            expect(r'^UNIT26_ACTUAL_EXIT=[1-9][0-9]*$', text, 'Privileged target did not fail')
        else:
            require('actual privileged inspection canceled and joined stopped root PID' in text,
                    'Restored test did not report joined actual child')
            expect(r'^UNIT26_ACTUAL_EXIT=0$', text, 'Restored privileged target failed')

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--session', default='session')
    args = parser.parse_args()
    require(Path.cwd().resolve() == ROOT, 'Run from the shared main checkout only')
    require(KERNEL.is_file(), 'The selected own-system kernel is unavailable')
    lock = (PLAN / 'proof-execution.lock').open('a+')
    fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
    rows = json.loads((PLAN / 'cuts.current.json').read_text())
    require(len(rows) == 27 and [int(row['cut'][:2]) for row in rows] == list(range(1, 28)),
            'Proof population must remain exactly cuts 01 through 27')
    for row in rows:
        require_hash(ROOT / row['source'], row['sha256'])
        require('MUTATION-APPLIED' not in (ROOT / row['source']).read_text(), 'An earlier cut is already active')
    stamp = datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ')
    output = Path(tempfile.mkdtemp(prefix='actual-all27-' + stamp + '-', dir=PLAN))
    save(output / 'plan.json', {'kernel': str(KERNEL), 'kernel_sha256': digest(KERNEL),
                               'cuts': rows, 'script_sha256': digest(Path(__file__))})
    executor = Executor(args.session, output)
    tags = subprocess.check_output(['bash', '-c', '. ./feature-tags\nprintf "ze_core,%s" "$(feature_tags feature-gates.txt ,)"'],
                                   cwd=ROOT, text=True).strip()
    require(tags.startswith('ze_core,'), 'Feature tags producer returned no current tag set')
    completed = []
    for row in rows:
        attempt = output / row['cut']
        attempt.mkdir()
        source = ROOT / row['source']
        patch = PLAN / row['patch']
        require_hash(source, row['sha256'])
        backup = attempt / 'source.before'
        shutil.copy2(source, backup)
        require_hash(backup, row['sha256'])
        shutil.copy2(patch, attempt / 'cut.patch')
        active_digest = None
        red_error = None
        restored = False
        try:
            # patch's preflight is exact and has no side effect; the actual
            # source mutation occurs only after a second current-source guard.
            subprocess.run(['patch', '--dry-run', '--batch', '--forward', '--fuzz=0', '-p1',
                            '-i', str(attempt / 'cut.patch')], cwd=ROOT, check=True)
            require_hash(source, row['sha256'])
            active_digest = proposed_digest(source, attempt / 'cut.patch')
            subprocess.run(['patch', '--batch', '--forward', '--fuzz=0', '-p1',
                            '-i', str(attempt / 'cut.patch')], cwd=ROOT, check=True)
            require_hash(source, active_digest)
            require(active_digest != row['sha256'], 'Cut produced an empty source diff')
            require('MUTATION-APPLIED' in source.read_text(), 'Applied cut lacks its source marker')
            result = subprocess.run(['diff', '-u', '--label', 'source.before', str(backup),
                                     '--label', row['source'], str(source)], text=True,
                                    stdout=subprocess.PIPE, cwd=ROOT)
            require(result.returncode == 1 and result.stdout, 'No nonempty exact mutation diff')
            (attempt / 'mutation.diff').write_text(result.stdout)
            save(attempt / 'digests.json', {'source_before': row['sha256'],
                'source_mutated': active_digest, 'patch': digest(patch), 'mutation': digest(attempt / 'mutation.diff')})
            try:
                stage(executor, row, attempt, 'red', tags)
            except Exception as error:
                red_error = str(error)
                save(attempt / 'red-invalid.json', {'error': red_error})
        finally:
            if active_digest is not None:
                require(not executor.unjoined, 'Unjoined job: refusing source restoration during possible build')
                require_hash(backup, row['sha256'])
                if digest(source) != row['sha256']:
                    require_hash(source, active_digest)
                    shutil.copy2(backup, source)
                require_hash(source, row['sha256'])
                restored = True
                save(attempt / 'restored.json', {'source_sha256': digest(source), 'exact_saved_bytes_restored': True})
        require(restored, 'Mutation/apply failed; inspect exact source before continuing')
        stage(executor, row, attempt, 'green', tags)
        require(red_error is None, 'RED was invalid; source restored and GREEN ran, but next cut is refused: ' + str(red_error))
        completed.append({'cut': row['cut'], 'semantic_red_validated': True,
                          'restored_green_validated': True, 'human_full_capture_review_owed': True})
        save(output / 'outcomes.json', completed)
        print('VERIFIED RED/RESTORED GREEN ' + row['cut'], flush=True)
    print('ALL 27 CUTS EXECUTED: retained evidence ' + str(output), flush=True)

def stage(executor, row, attempt, phase, tags):
    number = int(row['cut'][:2])
    if number <= 24:
        executor.daemon_stage(row, attempt, phase)
    elif number == 26:
        executor.privileged_stage(row, attempt, phase, tags)
    else:
        executor.unit_stage(row, attempt, phase, tags)

if __name__ == '__main__':
    signal.signal(signal.SIGTERM, lambda *_: (_ for _ in ()).throw(KeyboardInterrupt()))
    try:
        main()
    except (Exception, KeyboardInterrupt) as error:
        print('PROOF STOPPED: ' + str(error), file=sys.stderr, flush=True)
        sys.exit(1)
