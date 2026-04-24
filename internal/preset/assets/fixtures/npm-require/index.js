const pkgName = process.env.LIMIER_PACKAGE;
const version = process.env.LIMIER_VERSION_UNDER_TEST || "unknown";

if (!pkgName) {
  console.log("limier fixture: LIMIER_PACKAGE not set, skipping require");
  process.exit(0);
}

try {
  require(pkgName);
  console.log(`limier fixture: required ${pkgName}@${version}`);
} catch (err) {
  console.log(`limier fixture: require(${pkgName}) failed: ${err.message}`);
}
