/** 骨架屏占位块：高度必传，宽度默认撑满。 */
export default function Skeleton({ h, w }: { h: number | string; w?: number | string }) {
  return <div className="skeleton" style={{ height: h, width: w }} aria-hidden="true" />
}
