export interface TreeNode {
	name: string;
	path: string;
	isDir: boolean;
	children: TreeNode[];
}

export function buildFileTree(paths: string[]): TreeNode[] {
	const root: TreeNode[] = [];

	for (const filePath of paths.sort()) {
		const segments = filePath.split('/').filter(Boolean);
		if (segments.length === 0) continue;

		let current = root;
		let currentPath = '';

		for (let i = 0; i < segments.length; i++) {
			const segment = segments[i];
			const isLast = i === segments.length - 1;
			currentPath = currentPath ? `${currentPath}/${segment}` : segment;

			let node = current.find((n) => n.name === segment && n.isDir === !isLast);
			if (!node) {
				node = {
					name: segment,
					path: currentPath,
					isDir: !isLast,
					children: []
				};
				current.push(node);
			}

			if (isLast) {
				node.isDir = false;
				node.path = filePath;
			} else {
				current = node.children;
			}
		}
	}

	return sortTree(root);
}

function sortTree(nodes: TreeNode[]): TreeNode[] {
	nodes.sort((a, b) => {
		if (a.isDir !== b.isDir) {
			return a.isDir ? -1 : 1;
		}
		return a.name.localeCompare(b.name);
	});

	for (const node of nodes) {
		if (node.children.length > 0) {
			sortTree(node.children);
		}
	}

	return nodes;
}
