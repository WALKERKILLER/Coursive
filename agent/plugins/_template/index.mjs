// Template runtime: the pi entry the manifest names. `activate` runs once when
// the plugin loads, so actions and cleanup belong here; the default export runs
// for every new conversation and must stay free of side effects.
export function activate(coursive) {
	coursive.action("hello", async (args) => ({
		toast: `${coursive.settings().greeting ?? "Hello"} from the template (${args?.from ?? "action"})`,
	}));
}

export default function (_pi, _coursive) {
	// Register tools or hooks with Pi's extension API here.
}
