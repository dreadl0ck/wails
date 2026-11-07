import { afterEach, describe, expect, it, vi } from 'vitest';

const { call } = vi.hoisted(() => {
    vi.stubGlobal('document', { documentElement: { addEventListener() {} } });
    return { call: vi.fn(() => Promise.resolve()) };
});
vi.mock('./runtime.js', () => ({
    objectNames: { Window: 1 },
    newRuntimeCaller: () => call,
}));

import Window, { handleFileDragOutEnded } from './window';

afterEach(() => { vi.unstubAllGlobals(); call.mockClear(); });

describe('native file drag-out', () => {
    it('passes files and an optional preview to the native window', async () => {
        await Window.ArmFileDragOut(['/tmp/kick.wav', '/tmp/snare.wav']);
        expect(call).toHaveBeenCalledWith(53, {
            paths: ['/tmp/kick.wav', '/tmp/snare.wav'], imagePath: '',
        });
        await Window.DisarmFileDragOut();
        expect(call).toHaveBeenLastCalledWith(54);
    });

    it.each([true, false])('reports completion and cancellation: %s', performed => {
        const target = { dispatchEvent: vi.fn() };
        vi.stubGlobal('window', target);
        handleFileDragOutEnded(performed);
        const event = target.dispatchEvent.mock.calls[0][0];
        expect(event.type).toBe('wails:file-drag-out-ended');
        expect(event.detail).toEqual({ performed });
    });
});
