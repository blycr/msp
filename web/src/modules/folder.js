/**
 * 从扁平的 MediaItem 列表中提取文件夹树。
 */

export function getFolderContents(items, currentFolder) {
  if (!currentFolder) {
    const labels = new Set(items.map(i => i.shareLabel).filter(Boolean));
    return {
      folders: [...labels].sort().map(l => ({
        name: l,
        path: l,
        count: items.filter(i => i.shareLabel === l).length,
      })),
      files: [],
    };
  }

  const parts = currentFolder.split('/');
  const shareLabel = parts[0];
  const subPath = parts.slice(1).join('/');

  const shareItems = items.filter(i => i.shareLabel === shareLabel);

  const folderSet = new Set();
  const files = [];

  for (const item of shareItems) {
    const rel = item.relPath || item.name;
    if (!subPath) {
      const slash = rel.indexOf('/');
      if (slash > 0) {
        folderSet.add(rel.substring(0, slash));
      } else {
        files.push(item);
      }
    } else {
      if (!rel.startsWith(subPath + '/')) continue;
      const rest = rel.substring(subPath.length + 1);
      const slash = rest.indexOf('/');
      if (slash > 0) {
        folderSet.add(rest.substring(0, slash));
      } else {
        files.push(item);
      }
    }
  }

  const folders = [...folderSet].sort().map(name => ({
    name,
    path: currentFolder + '/' + name,
    count: 0,
  }));

  return { folders, files };
}
