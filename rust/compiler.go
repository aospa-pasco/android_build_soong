// Copyright 2019 The Android Open Source Project
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rust

import (
	"fmt"
	"path/filepath"

	"github.com/google/blueprint/proptools"

	"android/soong/android"
	"android/soong/rust/config"
)

func (compiler *baseCompiler) edition() string {
	return proptools.StringDefault(compiler.Properties.Edition, config.DefaultEdition)
}

func (compiler *baseCompiler) setNoStdlibs() {
	compiler.Properties.No_stdlibs = proptools.BoolPtr(true)
}

func (compiler *baseCompiler) disableLints() {
	compiler.Properties.Lints = proptools.StringPtr("none")
}

func NewBaseCompiler(dir, dir64 string, location installLocation) *baseCompiler {
	return &baseCompiler{
		Properties: BaseCompilerProperties{},
		dir:        dir,
		dir64:      dir64,
		location:   location,
	}
}

type installLocation int

const (
	InstallInSystem installLocation = 0
	InstallInData                   = iota

	incorrectSourcesError = "srcs can only contain one path for a rust file and source providers prefixed by \":\""
)

type BaseCompilerProperties struct {
	// path to the source file that is the main entry point of the program (e.g. main.rs or lib.rs)
	Srcs []string `android:"path,arch_variant"`

	// name of the lint set that should be used to validate this module.
	//
	// Possible values are "default" (for using a sensible set of lints
	// depending on the module's location), "android" (for the strictest
	// lint set that applies to all Android platform code), "vendor" (for
	// a relaxed set) and "none" (for ignoring all lint warnings and
	// errors). The default value is "default".
	Lints *string

	// flags to pass to rustc
	Flags []string `android:"path,arch_variant"`

	// flags to pass to the linker
	Ld_flags []string `android:"path,arch_variant"`

	// list of rust rlib crate dependencies
	Rlibs []string `android:"arch_variant"`

	// list of rust dylib crate dependencies
	Dylibs []string `android:"arch_variant"`

	// list of rust automatic crate dependencies
	Rustlibs []string `android:"arch_variant"`

	// list of rust proc_macro crate dependencies
	Proc_macros []string `android:"arch_variant"`

	// list of C shared library dependencies
	Shared_libs []string `android:"arch_variant"`

	// list of C static library dependencies
	Static_libs []string `android:"arch_variant"`

	// crate name, required for modules which produce Rust libraries: rust_library, rust_ffi and SourceProvider
	// modules which create library variants (rust_bindgen). This must be the expected extern crate name used in
	// source, and is required to conform to an enforced format matching library output files (if the output file is
	// lib<someName><suffix>, the crate_name property must be <someName>).
	Crate_name string `android:"arch_variant"`

	// list of features to enable for this crate
	Features []string `android:"arch_variant"`

	// specific rust edition that should be used if the default version is not desired
	Edition *string `android:"arch_variant"`

	// sets name of the output
	Stem *string `android:"arch_variant"`

	// append to name of output
	Suffix *string `android:"arch_variant"`

	// install to a subdirectory of the default install path for the module
	Relative_install_path *string `android:"arch_variant"`

	// whether to suppress inclusion of standard crates - defaults to false
	No_stdlibs *bool
}

type baseCompiler struct {
	Properties   BaseCompilerProperties
	coverageFile android.Path //rustc generates a single gcno file

	// Install related
	dir      string
	dir64    string
	subDir   string
	relative string
	path     android.InstallPath
	location installLocation

	coverageOutputZipFile android.OptionalPath
	distFile              android.OptionalPath
	// Stripped output file. If Valid(), this file will be installed instead of outputFile.
	strippedOutputFile android.OptionalPath
}

func (compiler *baseCompiler) Disabled() bool {
	return false
}

func (compiler *baseCompiler) SetDisabled() {
	panic("baseCompiler does not implement SetDisabled()")
}

func (compiler *baseCompiler) coverageOutputZipPath() android.OptionalPath {
	panic("baseCompiler does not implement coverageOutputZipPath()")
}

func (compiler *baseCompiler) staticStd(ctx *depsContext) bool {
	// For devices, we always link stdlibs in as dylibs by default.
	if ctx.Device() {
		return false
	} else {
		return true
	}
}

var _ compiler = (*baseCompiler)(nil)

func (compiler *baseCompiler) inData() bool {
	return compiler.location == InstallInData
}

func (compiler *baseCompiler) compilerProps() []interface{} {
	return []interface{}{&compiler.Properties}
}

func (compiler *baseCompiler) featuresToFlags(features []string) []string {
	flags := []string{}
	for _, feature := range features {
		flags = append(flags, "--cfg 'feature=\""+feature+"\"'")
	}
	return flags
}

func (compiler *baseCompiler) compilerFlags(ctx ModuleContext, flags Flags) Flags {

	lintFlags, err := config.RustcLintsForDir(ctx.ModuleDir(), compiler.Properties.Lints)
	if err != nil {
		ctx.PropertyErrorf("lints", err.Error())
	}
	flags.RustFlags = append(flags.RustFlags, lintFlags)
	flags.RustFlags = append(flags.RustFlags, compiler.Properties.Flags...)
	flags.RustFlags = append(flags.RustFlags, compiler.featuresToFlags(compiler.Properties.Features)...)
	flags.RustFlags = append(flags.RustFlags, "--edition="+compiler.edition())
	flags.LinkFlags = append(flags.LinkFlags, compiler.Properties.Ld_flags...)
	flags.GlobalRustFlags = append(flags.GlobalRustFlags, config.GlobalRustFlags...)
	flags.GlobalRustFlags = append(flags.GlobalRustFlags, ctx.toolchain().ToolchainRustFlags())
	flags.GlobalLinkFlags = append(flags.GlobalLinkFlags, ctx.toolchain().ToolchainLinkFlags())

	if ctx.Host() && !ctx.Windows() {
		rpath_prefix := `\$$ORIGIN/`
		if ctx.Darwin() {
			rpath_prefix = "@loader_path/"
		}

		var rpath string
		if ctx.toolchain().Is64Bit() {
			rpath = "lib64"
		} else {
			rpath = "lib"
		}
		flags.LinkFlags = append(flags.LinkFlags, "-Wl,-rpath,"+rpath_prefix+rpath)
		flags.LinkFlags = append(flags.LinkFlags, "-Wl,-rpath,"+rpath_prefix+"../"+rpath)
	}

	return flags
}

func (compiler *baseCompiler) compile(ctx ModuleContext, flags Flags, deps PathDeps) android.Path {
	panic(fmt.Errorf("baseCrater doesn't know how to crate things!"))
}

func (compiler *baseCompiler) compilerDeps(ctx DepsContext, deps Deps) Deps {
	deps.Rlibs = append(deps.Rlibs, compiler.Properties.Rlibs...)
	deps.Dylibs = append(deps.Dylibs, compiler.Properties.Dylibs...)
	deps.Rustlibs = append(deps.Rustlibs, compiler.Properties.Rustlibs...)
	deps.ProcMacros = append(deps.ProcMacros, compiler.Properties.Proc_macros...)
	deps.StaticLibs = append(deps.StaticLibs, compiler.Properties.Static_libs...)
	deps.SharedLibs = append(deps.SharedLibs, compiler.Properties.Shared_libs...)

	if !Bool(compiler.Properties.No_stdlibs) {
		for _, stdlib := range config.Stdlibs {
			// If we're building for the primary arch of the build host, use the compiler's stdlibs
			if ctx.Target().Os == android.BuildOs && ctx.TargetPrimary() {
				stdlib = stdlib + "_" + ctx.toolchain().RustTriple()
			}

			deps.Stdlibs = append(deps.Stdlibs, stdlib)
		}
	}
	return deps
}

func bionicDeps(deps Deps) Deps {
	deps.SharedLibs = append(deps.SharedLibs, "liblog")
	deps.SharedLibs = append(deps.SharedLibs, "libc")
	deps.SharedLibs = append(deps.SharedLibs, "libm")
	deps.SharedLibs = append(deps.SharedLibs, "libdl")

	//TODO(b/141331117) libstd requires libgcc on Android
	deps.StaticLibs = append(deps.StaticLibs, "libgcc")

	return deps
}

func (compiler *baseCompiler) crateName() string {
	return compiler.Properties.Crate_name
}

func (compiler *baseCompiler) installDir(ctx ModuleContext) android.InstallPath {
	dir := compiler.dir
	if ctx.toolchain().Is64Bit() && compiler.dir64 != "" {
		dir = compiler.dir64
	}
	if ctx.Target().NativeBridge == android.NativeBridgeEnabled {
		dir = filepath.Join(dir, ctx.Target().NativeBridgeRelativePath)
	}
	if !ctx.Host() && ctx.Config().HasMultilibConflict(ctx.Arch().ArchType) {
		dir = filepath.Join(dir, ctx.Arch().ArchType.String())
	}
	return android.PathForModuleInstall(ctx, dir, compiler.subDir,
		compiler.relativeInstallPath(), compiler.relative)
}

func (compiler *baseCompiler) nativeCoverage() bool {
	return false
}

func (compiler *baseCompiler) install(ctx ModuleContext) {
	path := ctx.RustModule().outputFile
	if compiler.strippedOutputFile.Valid() {
		path = compiler.strippedOutputFile
	}
	compiler.path = ctx.InstallFile(compiler.installDir(ctx), path.Path().Base(), path.Path())
}

func (compiler *baseCompiler) getStem(ctx ModuleContext) string {
	return compiler.getStemWithoutSuffix(ctx) + String(compiler.Properties.Suffix)
}

func (compiler *baseCompiler) getStemWithoutSuffix(ctx BaseModuleContext) string {
	stem := ctx.ModuleName()
	if String(compiler.Properties.Stem) != "" {
		stem = String(compiler.Properties.Stem)
	}

	return stem
}

func (compiler *baseCompiler) relativeInstallPath() string {
	return String(compiler.Properties.Relative_install_path)
}

// Returns the Path for the main source file along with Paths for generated source files from modules listed in srcs.
func srcPathFromModuleSrcs(ctx ModuleContext, srcs []string) (android.Path, android.Paths) {
	// The srcs can contain strings with prefix ":".
	// They are dependent modules of this module, with android.SourceDepTag.
	// They are not the main source file compiled by rustc.
	numSrcs := 0
	srcIndex := 0
	for i, s := range srcs {
		if android.SrcIsModule(s) == "" {
			numSrcs++
			srcIndex = i
		}
	}
	if numSrcs != 1 {
		ctx.PropertyErrorf("srcs", incorrectSourcesError)
	}
	if srcIndex != 0 {
		ctx.PropertyErrorf("srcs", "main source file must be the first in srcs")
	}
	paths := android.PathsForModuleSrc(ctx, srcs)
	return paths[srcIndex], paths[1:]
}
