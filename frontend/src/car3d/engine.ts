// Самописный WebGL-движок (без библиотек): low-poly машина 5 типов кузова, металлик-шейдер
// со specular-бликом, вращение + drag. Фабрика createCarScene создаёт независимые сцены
// (главная на hero + мини-превью в модалке).

import type { SceneBody as BodyType } from '@/data/presets'

export type RGB = [number, number, number]

export interface CarSceneOptions {
  interactive?: boolean
  autoSpin?: boolean
  dist?: number
  camY?: number
  tilt?: number
  type?: BodyType
  color?: RGB
  /** Чёрный силуэт вместо цветного рендера (плейсхолдер для пустого гаража). */
  silhouette?: boolean
}

export interface CarScene {
  setType(type: BodyType): void
  setColorRGB(rgb: RGB): void
  setSilhouette(on: boolean): void
  flourish(): void
  resize(): void
  destroy(): void
}

type Vec3 = [number, number, number]
type Mat4 = number[]

const reduceMotion =
  typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches

export function createCarScene(canvas: HTMLCanvasElement, opts: CarSceneOptions = {}): CarScene | null {
  let gl: WebGLRenderingContext | null = null
  try {
    gl = canvas.getContext('webgl', { antialias: true, alpha: true, premultipliedAlpha: false })
  } catch {
    gl = null
  }
  if (!gl) {
    canvas.style.display = 'none'
    return null
  }
  const g = gl

  /* ---- mat4 (column-major) ---- */
  const ident = (): Mat4 => [1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1]
  function mul(a: Mat4, b: Mat4): Mat4 {
    const o = new Array(16)
    for (let c = 0; c < 4; c++)
      for (let r = 0; r < 4; r++)
        o[c * 4 + r] =
          a[r] * b[c * 4] + a[4 + r] * b[c * 4 + 1] + a[8 + r] * b[c * 4 + 2] + a[12 + r] * b[c * 4 + 3]
    return o
  }
  function perspective(fovy: number, asp: number, n: number, f: number): Mat4 {
    const t = 1 / Math.tan(fovy / 2)
    return [t / asp, 0, 0, 0, 0, t, 0, 0, 0, 0, (f + n) / (n - f), -1, 0, 0, (2 * f * n) / (n - f), 0]
  }
  function trans(x: number, y: number, z: number): Mat4 {
    const m = ident()
    m[12] = x
    m[13] = y
    m[14] = z
    return m
  }
  function rotX(a: number): Mat4 {
    const c = Math.cos(a), s = Math.sin(a)
    return [1, 0, 0, 0, 0, c, s, 0, 0, -s, c, 0, 0, 0, 0, 1]
  }
  function rotY(a: number): Mat4 {
    const c = Math.cos(a), s = Math.sin(a)
    return [c, 0, -s, 0, 0, 1, 0, 0, s, 0, c, 0, 0, 0, 0, 1]
  }

  /* ---- geometry buffers ---- */
  let P: number[] = [], N: number[] = [], C: number[] = [], E: number[] = [], M: number[] = []
  let vCount = 0
  function resetGeo(): void {
    P = []; N = []; C = []; E = []; M = []
  }
  function push(px: number, py: number, pz: number, nx: number, ny: number, nz: number, col: Vec3, em: number, mask: number): void {
    P.push(px, py, pz); N.push(nx, ny, nz); C.push(col[0], col[1], col[2]); E.push(em); M.push(mask)
  }
  function face(v0: Vec3, v1: Vec3, v2: Vec3, v3: Vec3, n: Vec3, col: Vec3, em: number, mask: number): void {
    push(v0[0], v0[1], v0[2], n[0], n[1], n[2], col, em, mask)
    push(v1[0], v1[1], v1[2], n[0], n[1], n[2], col, em, mask)
    push(v2[0], v2[1], v2[2], n[0], n[1], n[2], col, em, mask)
    push(v0[0], v0[1], v0[2], n[0], n[1], n[2], col, em, mask)
    push(v2[0], v2[1], v2[2], n[0], n[1], n[2], col, em, mask)
    push(v3[0], v3[1], v3[2], n[0], n[1], n[2], col, em, mask)
  }
  function box(cx: number, cy: number, cz: number, sx: number, sy: number, sz: number, col: Vec3, em = 0, mask = 0): void {
    const x = sx / 2, y = sy / 2, z = sz / 2
    const A: Vec3 = [cx - x, cy - y, cz - z], B: Vec3 = [cx + x, cy - y, cz - z], Cc: Vec3 = [cx + x, cy + y, cz - z], D: Vec3 = [cx - x, cy + y, cz - z]
    const E2: Vec3 = [cx - x, cy - y, cz + z], F: Vec3 = [cx + x, cy - y, cz + z], G: Vec3 = [cx + x, cy + y, cz + z], H: Vec3 = [cx - x, cy + y, cz + z]
    face(F, B, Cc, G, [1, 0, 0], col, em, mask)
    face(A, E2, H, D, [-1, 0, 0], col, em, mask)
    face(H, G, Cc, D, [0, 1, 0], col, em, mask)
    face(A, B, F, E2, [0, -1, 0], col, em, mask)
    face(E2, F, G, H, [0, 0, 1], col, em, mask)
    face(B, A, D, Cc, [0, 0, -1], col, em, mask)
  }
  function wheel(cx: number, cy: number, cz: number, r: number, w: number, col: Vec3, rim: Vec3): void {
    const seg = 16, hx = w / 2
    for (let i = 0; i < seg; i++) {
      const a0 = (i / seg) * Math.PI * 2, a1 = ((i + 1) / seg) * Math.PI * 2, am = (a0 + a1) / 2
      const y0 = Math.cos(a0) * r, z0 = Math.sin(a0) * r, y1 = Math.cos(a1) * r, z1 = Math.sin(a1) * r
      const nm: Vec3 = [0, Math.cos(am), Math.sin(am)]
      face([cx - hx, cy + y0, cz + z0], [cx + hx, cy + y0, cz + z0], [cx + hx, cy + y1, cz + z1], [cx - hx, cy + y1, cz + z1], nm, col, 0, 0)
      push(cx + hx, cy, cz, 1, 0, 0, rim, 0, 0); push(cx + hx, cy + y0, cz + z0, 1, 0, 0, rim, 0, 0); push(cx + hx, cy + y1, cz + z1, 1, 0, 0, rim, 0, 0)
      push(cx - hx, cy, cz, -1, 0, 0, col, 0, 0); push(cx - hx, cy + y1, cz + z1, -1, 0, 0, col, 0, 0); push(cx - hx, cy + y0, cz + z0, -1, 0, 0, col, 0, 0)
    }
  }
  const sub = (a: Vec3, b: Vec3): Vec3 => [a[0] - b[0], a[1] - b[1], a[2] - b[2]]
  const crs = (a: Vec3, b: Vec3): Vec3 => [a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0]]
  function faceAuto(v0: Vec3, v1: Vec3, v2: Vec3, v3: Vec3, ct: Vec3, col: Vec3, em: number, mask: number): void {
    let n = crs(sub(v1, v0), sub(v2, v0))
    const mid: Vec3 = [(v0[0] + v1[0] + v2[0] + v3[0]) / 4, (v0[1] + v1[1] + v2[1] + v3[1]) / 4, (v0[2] + v1[2] + v2[2] + v3[2]) / 4]
    const o = sub(mid, ct)
    if (n[0] * o[0] + n[1] * o[1] + n[2] * o[2] < 0) n = [-n[0], -n[1], -n[2]]
    const L = Math.hypot(n[0], n[1], n[2]) || 1
    face(v0, v1, v2, v3, [n[0] / L, n[1] / L, n[2] / L], col, em, mask)
  }
  // блок из 8 вершин: 0-3 низ (-x-z,+x-z,+x+z,-x+z), 4-7 верх (тот же порядок)
  function block8(c: Vec3[], col: Vec3, em = 0, mask = 0): void {
    const ct: Vec3 = [0, 0, 0]
    for (let i = 0; i < 8; i++) { ct[0] += c[i][0]; ct[1] += c[i][1]; ct[2] += c[i][2] }
    ct[0] /= 8; ct[1] /= 8; ct[2] /= 8
    faceAuto(c[0], c[1], c[2], c[3], ct, col, em, mask); faceAuto(c[4], c[5], c[6], c[7], ct, col, em, mask)
    faceAuto(c[0], c[1], c[5], c[4], ct, col, em, mask); faceAuto(c[3], c[2], c[6], c[7], ct, col, em, mask)
    faceAuto(c[1], c[2], c[6], c[5], ct, col, em, mask); faceAuto(c[0], c[3], c[7], c[4], ct, col, em, mask)
  }

  const WHITE: Vec3 = [1, 1, 1], GLASS: Vec3 = [0.05, 0.08, 0.12], TIRE: Vec3 = [0.05, 0.06, 0.08],
    RIM: Vec3 = [0.58, 0.63, 0.72], HEAD: Vec3 = [1, 0.96, 0.8], TAIL: Vec3 = [1, 0.22, 0.2],
    TRIM: Vec3 = [0.09, 0.1, 0.13], CHROME: Vec3 = [0.72, 0.77, 0.84]

  interface Extras {
    rockerY: number; rockerLen: number; mirrorY: number; mirrorZ: number; lightY: number
    fz: number; rz: number; wx: number; wz: number; wr: number; ww: number; ry: number
  }
  function extras(o: Extras): void {
    box(0, o.rockerY, 0, 1.9, 0.12, o.rockerLen, TRIM, 0, 0)
    box(0.95, o.mirrorY, o.mirrorZ, 0.18, 0.09, 0.16, WHITE, 0, 1); box(-0.95, o.mirrorY, o.mirrorZ, 0.18, 0.09, 0.16, WHITE, 0, 1)
    box(0.58, o.lightY, o.fz - 0.02, 0.42, 0.18, 0.06, HEAD, 1, 0); box(-0.58, o.lightY, o.fz - 0.02, 0.42, 0.18, 0.06, HEAD, 1, 0)
    box(0.6, o.lightY, o.rz + 0.02, 0.4, 0.16, 0.05, TAIL, 0.85, 0); box(-0.6, o.lightY, o.rz + 0.02, 0.4, 0.16, 0.05, TAIL, 0.85, 0)
    box(0, o.lightY - 0.06, o.fz, 1.05, 0.24, 0.06, TRIM, 0, 0)
    box(0, o.lightY - 0.04, o.fz + 0.02, 0.55, 0.05, 0.05, CHROME, 0, 0)
    wheel(o.wx, o.ry, o.wz, o.wr, o.ww, TIRE, RIM); wheel(-o.wx, o.ry, o.wz, o.wr, o.ww, TIRE, RIM)
    wheel(o.wx, o.ry, -o.wz, o.wr, o.ww, TIRE, RIM); wheel(-o.wx, o.ry, -o.wz, o.wr, o.ww, TIRE, RIM)
  }

  const builders: Record<BodyType, () => void> = {
    sedan() {
      block8([[-0.92, 0.14, -2.0], [0.92, 0.14, -2.0], [0.92, 0.14, 2.0], [-0.92, 0.14, 2.0], [-0.92, 0.6, -2.0], [0.92, 0.6, -2.0], [0.92, 0.6, 2.0], [-0.92, 0.6, 2.0]], WHITE, 0, 1)
      block8([[-0.82, 0.56, 0.3], [0.82, 0.56, 0.3], [0.82, 0.56, 1.98], [-0.82, 0.56, 1.98], [-0.82, 0.82, 0.3], [0.82, 0.82, 0.3], [0.82, 0.66, 1.98], [-0.82, 0.66, 1.98]], WHITE, 0, 1)
      block8([[-0.82, 0.56, -1.98], [0.82, 0.56, -1.98], [0.82, 0.56, -0.4], [-0.82, 0.56, -0.4], [-0.82, 0.66, -1.98], [0.82, 0.66, -1.98], [0.82, 0.82, -0.4], [-0.82, 0.82, -0.4]], WHITE, 0, 1)
      block8([[-0.8, 0.6, -1.05], [0.8, 0.6, -1.05], [0.8, 0.6, 0.55], [-0.8, 0.6, 0.55], [-0.58, 1.2, -0.75], [0.58, 1.2, -0.75], [0.58, 1.2, 0.02], [-0.58, 1.2, 0.02]], GLASS, 0, 0)
      block8([[-0.58, 1.16, -0.75], [0.58, 1.16, -0.75], [0.58, 1.16, 0.02], [-0.58, 1.16, 0.02], [-0.56, 1.26, -0.72], [0.56, 1.26, -0.72], [0.56, 1.26, 0], [-0.56, 1.26, 0]], WHITE, 0, 1)
      extras({ rockerY: 0.18, rockerLen: 3.9, mirrorY: 0.9, mirrorZ: 0.42, lightY: 0.54, fz: 2.0, rz: -2.0, wx: 0.92, wz: 1.3, wr: 0.5, ww: 0.34, ry: 0.06 })
    },
    hatch() {
      block8([[-0.9, 0.14, -1.7], [0.9, 0.14, -1.7], [0.9, 0.14, 1.9], [-0.9, 0.14, 1.9], [-0.9, 0.6, -1.7], [0.9, 0.6, -1.7], [0.9, 0.6, 1.9], [-0.9, 0.6, 1.9]], WHITE, 0, 1)
      block8([[-0.8, 0.56, 0.35], [0.8, 0.56, 0.35], [0.8, 0.56, 1.88], [-0.8, 0.56, 1.88], [-0.8, 0.82, 0.35], [0.8, 0.82, 0.35], [0.8, 0.66, 1.88], [-0.8, 0.66, 1.88]], WHITE, 0, 1)
      block8([[-0.8, 0.6, -1.6], [0.8, 0.6, -1.6], [0.8, 0.6, 0.55], [-0.8, 0.6, 0.55], [-0.58, 1.26, -1.42], [0.58, 1.26, -1.42], [0.58, 1.26, 0.05], [-0.58, 1.26, 0.05]], GLASS, 0, 0)
      block8([[-0.58, 1.22, -1.42], [0.58, 1.22, -1.42], [0.58, 1.22, 0.05], [-0.58, 1.22, 0.05], [-0.56, 1.32, -1.4], [0.56, 1.32, -1.4], [0.56, 1.32, 0.03], [-0.56, 1.32, 0.03]], WHITE, 0, 1)
      extras({ rockerY: 0.18, rockerLen: 3.5, mirrorY: 0.92, mirrorZ: 0.45, lightY: 0.54, fz: 1.9, rz: -1.7, wx: 0.9, wz: 1.18, wr: 0.5, ww: 0.34, ry: 0.06 })
    },
    suv() {
      block8([[-0.95, 0.24, -2.05], [0.95, 0.24, -2.05], [0.95, 0.24, 2.05], [-0.95, 0.24, 2.05], [-0.95, 0.86, -2.05], [0.95, 0.86, -2.05], [0.95, 0.86, 2.05], [-0.95, 0.86, 2.05]], WHITE, 0, 1)
      block8([[-0.86, 0.82, 0.4], [0.86, 0.82, 0.4], [0.86, 0.82, 2.0], [-0.86, 0.82, 2.0], [-0.86, 1.0, 0.4], [0.86, 1.0, 0.4], [0.86, 0.92, 2.0], [-0.86, 0.92, 2.0]], WHITE, 0, 1)
      block8([[-0.85, 0.86, -1.7], [0.85, 0.86, -1.7], [0.85, 0.86, 0.5], [-0.85, 0.86, 0.5], [-0.7, 1.5, -1.55], [0.7, 1.5, -1.55], [0.7, 1.5, 0.28], [-0.7, 1.5, 0.28]], GLASS, 0, 0)
      block8([[-0.7, 1.46, -1.55], [0.7, 1.46, -1.55], [0.7, 1.46, 0.28], [-0.7, 1.46, 0.28], [-0.68, 1.58, -1.5], [0.68, 1.58, -1.5], [0.68, 1.58, 0.24], [-0.68, 1.58, 0.24]], WHITE, 0, 1)
      extras({ rockerY: 0.28, rockerLen: 3.9, mirrorY: 1.15, mirrorZ: 0.35, lightY: 0.7, fz: 2.05, rz: -2.05, wx: 0.96, wz: 1.4, wr: 0.6, ww: 0.4, ry: 0.14 })
    },
    coupe() {
      block8([[-0.92, 0.1, -1.95], [0.92, 0.1, -1.95], [0.92, 0.1, 2.0], [-0.92, 0.1, 2.0], [-0.92, 0.5, -1.95], [0.92, 0.5, -1.95], [0.92, 0.5, 2.0], [-0.92, 0.5, 2.0]], WHITE, 0, 1)
      block8([[-0.82, 0.46, 0.4], [0.82, 0.46, 0.4], [0.82, 0.46, 1.98], [-0.82, 0.46, 1.98], [-0.82, 0.66, 0.4], [0.82, 0.66, 0.4], [0.82, 0.52, 1.98], [-0.82, 0.52, 1.98]], WHITE, 0, 1)
      block8([[-0.8, 0.5, -1.6], [0.8, 0.5, -1.6], [0.8, 0.5, 0.6], [-0.8, 0.5, 0.6], [-0.54, 1.02, -0.5], [0.54, 1.02, -0.5], [0.54, 1.02, 0.15], [-0.54, 1.02, 0.15]], GLASS, 0, 0)
      block8([[-0.54, 0.98, -0.5], [0.54, 0.98, -0.5], [0.54, 0.98, 0.15], [-0.54, 0.98, 0.15], [-0.52, 1.08, -0.48], [0.52, 1.08, -0.48], [0.52, 1.08, 0.12], [-0.52, 1.08, 0.12]], WHITE, 0, 1)
      extras({ rockerY: 0.14, rockerLen: 3.8, mirrorY: 0.76, mirrorZ: 0.5, lightY: 0.46, fz: 2.0, rz: -1.95, wx: 0.92, wz: 1.28, wr: 0.52, ww: 0.36, ry: 0.02 })
    },
    pickup() {
      block8([[-0.94, 0.22, -2.1], [0.94, 0.22, -2.1], [0.94, 0.22, 2.1], [-0.94, 0.22, 2.1], [-0.94, 0.78, -2.1], [0.94, 0.78, -2.1], [0.94, 0.78, 2.1], [-0.94, 0.78, 2.1]], WHITE, 0, 1)
      block8([[-0.86, 0.74, 0.5], [0.86, 0.74, 0.5], [0.86, 0.74, 2.05], [-0.86, 0.74, 2.05], [-0.86, 0.94, 0.5], [0.86, 0.94, 0.5], [0.86, 0.84, 2.05], [-0.86, 0.84, 2.05]], WHITE, 0, 1)
      block8([[-0.84, 0.78, -0.2], [0.84, 0.78, -0.2], [0.84, 0.78, 0.6], [-0.84, 0.78, 0.6], [-0.66, 1.4, -0.05], [0.66, 1.4, -0.05], [0.66, 1.4, 0.4], [-0.66, 1.4, 0.4]], GLASS, 0, 0)
      block8([[-0.66, 1.36, -0.05], [0.66, 1.36, -0.05], [0.66, 1.36, 0.4], [-0.66, 1.36, 0.4], [-0.64, 1.46, -0.03], [0.64, 1.46, -0.03], [0.64, 1.46, 0.38], [-0.64, 1.46, 0.38]], WHITE, 0, 1)
      box(0, 0.9, -0.35, 1.7, 0.24, 0.1, WHITE, 0, 1)
      box(0.82, 0.98, -1.25, 0.16, 0.42, 1.7, WHITE, 0, 1); box(-0.82, 0.98, -1.25, 0.16, 0.42, 1.7, WHITE, 0, 1)
      box(0, 0.9, -2.1, 1.7, 0.34, 0.1, WHITE, 0, 1)
      box(0, 0.66, -1.25, 1.5, 0.04, 1.6, TRIM, 0, 0)
      extras({ rockerY: 0.26, rockerLen: 4.0, mirrorY: 1.05, mirrorZ: 0.15, lightY: 0.62, fz: 2.1, rz: -2.1, wx: 0.95, wz: 1.5, wr: 0.6, ww: 0.4, ry: 0.14 })
    },
  }

  /* ---- shaders ---- */
  const vsrc =
    'attribute vec3 aPos; attribute vec3 aNorm; attribute vec3 aCol; attribute float aEm; attribute float aMask;' +
    'uniform mat4 uProj; uniform mat4 uMV; uniform vec3 uBody;' +
    'varying vec3 vN; varying vec3 vCol; varying float vEm; varying vec3 vPos;' +
    'void main(){ vec4 p = uMV * vec4(aPos,1.0); vPos = p.xyz; gl_Position = uProj * p;' +
    ' vN = mat3(uMV) * aNorm; vCol = mix(aCol, uBody, aMask); vEm = aEm; }'
  const fsrc =
    'precision mediump float;' +
    'uniform float uSil;' +
    'varying vec3 vN; varying vec3 vCol; varying float vEm; varying vec3 vPos;' +
    'void main(){ vec3 N = normalize(vN); vec3 V = normalize(-vPos);' +
    ' vec3 L1 = normalize(vec3(0.4,0.78,0.55)); vec3 L2 = normalize(vec3(-0.55,0.4,-0.35));' +
    ' float d1 = max(dot(N,L1),0.0); float d2 = max(dot(N,L2),0.0);' +
    ' vec3 H = normalize(L1+V); float spec = pow(max(dot(N,H),0.0), 42.0) * 0.6;' +
    ' float rim = pow(1.0 - max(dot(N,V),0.0), 3.0);' +
    ' vec3 lit = vCol * (0.24 + 0.82*d1 + 0.22*d2) + vec3(1.0)*spec + vec3(0.2,0.5,0.6)*rim*0.28;' +
    ' vec3 col = mix(lit, vCol, vEm);' +
    ' vec3 sil = vec3(0.015,0.02,0.03) + vec3(0.2,0.5,0.6)*rim*0.55;' +
    ' col = mix(col, sil, uSil);' +
    ' gl_FragColor = vec4(col, 1.0); }'

  function sh(type: number, src: string): WebGLShader {
    const s = g.createShader(type)!
    g.shaderSource(s, src)
    g.compileShader(s)
    if (!g.getShaderParameter(s, g.COMPILE_STATUS)) console.warn(g.getShaderInfoLog(s))
    return s
  }
  const prog = g.createProgram()!
  g.attachShader(prog, sh(g.VERTEX_SHADER, vsrc))
  g.attachShader(prog, sh(g.FRAGMENT_SHADER, fsrc))
  g.linkProgram(prog)
  g.useProgram(prog)

  const attrCache: Record<string, WebGLBuffer> = {}
  function uploadAttr(name: string, data: number[], size: number): void {
    const b = attrCache[name] || (attrCache[name] = g.createBuffer()!)
    g.bindBuffer(g.ARRAY_BUFFER, b)
    g.bufferData(g.ARRAY_BUFFER, new Float32Array(data), g.STATIC_DRAW)
    const loc = g.getAttribLocation(prog, name)
    g.enableVertexAttribArray(loc)
    g.vertexAttribPointer(loc, size, g.FLOAT, false, 0, 0)
  }
  function buildCar(type: BodyType): void {
    resetGeo()
    ;(builders[type] || builders.sedan)()
    vCount = P.length / 3
    uploadAttr('aPos', P, 3); uploadAttr('aNorm', N, 3); uploadAttr('aCol', C, 3)
    uploadAttr('aEm', E, 1); uploadAttr('aMask', M, 1)
  }
  buildCar(opts.type ?? 'sedan')

  const uProj = g.getUniformLocation(prog, 'uProj')
  const uMV = g.getUniformLocation(prog, 'uMV')
  const uBody = g.getUniformLocation(prog, 'uBody')
  const uSil = g.getUniformLocation(prog, 'uSil')
  g.enable(g.DEPTH_TEST)
  g.clearColor(0, 0, 0, 0)

  let bodyColor: RGB = opts.color ? [opts.color[0], opts.color[1], opts.color[2]] : [0.16, 0.5, 0.56]
  let silhouette = opts.silhouette ? 1 : 0

  /* ---- interaction ---- */
  let spin = 0.6, velY = 0, dragging = false, lastX = 0, flourishAmt = 0
  const autoSpin = reduceMotion ? 0 : opts.autoSpin === false ? 0 : 0.0032
  const tilt = opts.tilt ?? 0.16
  const dist = opts.dist ?? 11.5
  const camY = opts.camY ?? -0.1

  const onDown = (e: PointerEvent) => { dragging = true; lastX = e.clientX; velY = 0; canvas.setPointerCapture(e.pointerId) }
  const onMove = (e: PointerEvent) => { if (!dragging) return; const dx = e.clientX - lastX; lastX = e.clientX; velY = dx * 0.01; spin += velY }
  const onUp = () => { dragging = false }
  if (opts.interactive) {
    canvas.addEventListener('pointerdown', onDown)
    canvas.addEventListener('pointermove', onMove)
    canvas.addEventListener('pointerup', onUp)
    canvas.addEventListener('pointercancel', onUp)
  }

  function resize(): void {
    const dpr = Math.min(window.devicePixelRatio || 1, 2)
    const w = canvas.clientWidth, h = canvas.clientHeight
    canvas.width = Math.floor(w * dpr)
    canvas.height = Math.floor(h * dpr)
    g.viewport(0, 0, canvas.width, canvas.height)
  }
  window.addEventListener('resize', resize)
  resize()

  let raf = 0
  function frame(): void {
    if (!dragging) { spin += autoSpin + velY; velY *= 0.94 }
    if (flourishAmt > 0) { const step = Math.min(flourishAmt, 0.14); spin += step; flourishAmt -= step }
    const asp = canvas.width / canvas.height || 1
    const proj = perspective((38 * Math.PI) / 180, asp, 0.1, 100)
    const mv = mul(trans(0, camY, -dist), mul(rotX(tilt), mul(rotY(spin), trans(0, -0.5, 0))))
    g.clear(g.COLOR_BUFFER_BIT | g.DEPTH_BUFFER_BIT)
    g.uniformMatrix4fv(uProj, false, new Float32Array(proj))
    g.uniformMatrix4fv(uMV, false, new Float32Array(mv))
    g.uniform3fv(uBody, new Float32Array(bodyColor))
    g.uniform1f(uSil, silhouette)
    g.drawArrays(g.TRIANGLES, 0, vCount)
    raf = requestAnimationFrame(frame)
  }
  raf = requestAnimationFrame(frame)

  return {
    setType(type: BodyType) { buildCar(type) },
    setColorRGB(rgb: RGB) { if (rgb) bodyColor = [rgb[0], rgb[1], rgb[2]] },
    setSilhouette(on: boolean) { silhouette = on ? 1 : 0 },
    flourish() { flourishAmt = Math.PI * 2 },
    resize,
    destroy() {
      cancelAnimationFrame(raf)
      window.removeEventListener('resize', resize)
      if (opts.interactive) {
        canvas.removeEventListener('pointerdown', onDown)
        canvas.removeEventListener('pointermove', onMove)
        canvas.removeEventListener('pointerup', onUp)
        canvas.removeEventListener('pointercancel', onUp)
      }
    },
  }
}
