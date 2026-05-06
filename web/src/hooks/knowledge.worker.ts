import { Delaunay } from 'd3-delaunay';

self.onmessage = (e) => {
  const { notes, clusters, width, height } = e.data;
  if (!notes || !clusters) return;

  // 1. Spatial Grid for fast lookups
  const cellSize = 10;
  const grid = new Map();
  notes.forEach((note: any) => {
    const gx = Math.floor(note.x / cellSize);
    const gy = Math.floor(note.y / cellSize);
    const key = `${gx},${gy}`;
    if (!grid.has(key)) grid.set(key, []);
    grid.get(key).push(note);
  });

  // 2. Voronoi for territories
  let voronoiPaths: (string | null)[] = [];
  if (clusters.length >= 2) {
    const minX = Math.min(...notes.map((n: any) => n.x)) - 200;
    const maxX = Math.max(...notes.map((n: any) => n.x)) + 200;
    const minY = Math.min(...notes.map((n: any) => n.y)) - 200;
    const maxY = Math.max(...notes.map((n: any) => n.y)) + 200;

    const points = clusters.map((c: any) => [c.x, c.y]);
    const delaunay = Delaunay.from(points);
    const voronoi = delaunay.voronoi([minX, minY, maxX, maxY]);

    voronoiPaths = clusters.map((_: any, i: number) => voronoi.renderCell(i));
  }

  self.postMessage({
    grid,
    cellSize,
    voronoiPaths,
  });
};
