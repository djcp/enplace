package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/djcp/gorecipes/internal/db"
	"github.com/djcp/gorecipes/internal/export"
	"github.com/djcp/gorecipes/internal/ui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	listQuery  string
	listStatus string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Browse and search recipes",
	Long: `Open the interactive recipe browser.

Use / to search, arrow keys to navigate, enter to view a recipe.`,
	RunE: runList,
}

func init() {
	listCmd.Flags().StringVarP(&listQuery, "query", "q", "", "Filter by name or ingredient (non-interactive)")
	listCmd.Flags().StringVarP(&listStatus, "status", "s", "", "Filter by status (published, review, draft, etc.)")
}

func runList(_ *cobra.Command, _ []string) error {
	filter := db.RecipeFilter{
		Query:        listQuery,
		StatusFilter: listStatus,
	}

	// Non-interactive path: flag query set, or stdout is not a TTY.
	if listQuery != "" || !isTerminal() {
		recipes, err := db.ListRecipes(sqlDB, filter)
		if err != nil {
			return fmt.Errorf("loading recipes: %w", err)
		}
		if len(recipes) == 0 {
			fmt.Println(ui.MutedStyle.Render("\n  No recipes found."))
			fmt.Println(ui.MutedStyle.Render("  Add one with: gorecipes add <url>"))
			fmt.Println()
			return nil
		}
		fmt.Printf("\n  Found %d recipe(s):\n\n", len(recipes))
		for _, r := range recipes {
			courses := strings.Join(r.TagsByContext("courses"), ", ")
			fmt.Printf("  %3d  %-40s  %s\n", r.ID, r.Name, ui.MutedStyle.Render(courses))
		}
		fmt.Println()
		return nil
	}

	// Load autocomplete data once; refreshed after each successful edit.
	editData, err := loadEditData()
	if err != nil {
		return fmt.Errorf("loading edit data: %w", err)
	}
	searchData, err := loadSearchData()
	if err != nil {
		return fmt.Errorf("loading search data: %w", err)
	}

	// pendingDetailID, when > 0, skips the list view and opens this recipe's
	// detail view directly (used after editing from the detail view).
	var pendingDetailID int64

	// Interactive path: loop between the list browser and recipe detail view.
	for {
		recipes, err := db.ListRecipes(sqlDB, filter)
		if err != nil {
			return fmt.Errorf("loading recipes: %w", err)
		}

		var selectedID int64

		if pendingDetailID > 0 {
			// Skip the list and go straight to the detail view.
			selectedID = pendingDetailID
			pendingDetailID = 0
		} else {
			var goAdd, goHome, goManage, searchConfirmed bool
			var filterState ui.FilterState
			var deleteID, editID int64

			selectedID, goAdd, goHome, searchConfirmed, filterState, deleteID, editID, goManage, err = ui.RunListUI(
				recipes,
				ui.FilterState{
					Query:      filter.Query,
					Courses:    filter.Courses,
					Influences: filter.CulturalInfluences,
					Status:     filter.StatusFilter,
				},
				searchData,
			)
			if err != nil {
				return err
			}
			if goHome {
				filter = db.RecipeFilter{}
				continue
			}
			if searchConfirmed {
				filter = db.RecipeFilter{
					Query:              filterState.Query,
					Courses:            filterState.Courses,
					CulturalInfluences: filterState.Influences,
					StatusFilter:       filterState.Status,
				}
				continue
			}
			if goManage {
				if err := runManageUI(); err != nil {
					return err
				}
				searchData, _ = loadSearchData()
				continue
			}
			if deleteID > 0 {
				if err := db.DeleteRecipe(sqlDB, deleteID); err != nil {
					return fmt.Errorf("deleting recipe: %w", err)
				}
				continue
			}
			if editID > 0 {
				recipeToEdit, err := db.GetRecipe(sqlDB, editID)
				if err != nil {
					return err
				}
				toSave, tagNames, _, err := ui.RunEditUI(recipeToEdit, editData)
				if err != nil {
					return err
				}
				if toSave != nil {
					if err := db.SaveRecipe(sqlDB, toSave, tagNames); err != nil {
						return fmt.Errorf("saving recipe: %w", err)
					}
					editData, _ = loadEditData()
					searchData, _ = loadSearchData()
				}
				continue
			}
			if goAdd {
				return runAdd(nil, nil)
			}
			if selectedID == 0 {
				break
			}
		}

		recipe, err := db.GetRecipe(sqlDB, selectedID)
		if err != nil {
			return err
		}

		goHome, goAdd, goEdit, goPrint, goManage, goRetry, deleteConfirmed, searchQuery, err := ui.RunDetailUI(recipe)
		if err != nil {
			return err
		}
		if deleteConfirmed {
			if err := db.DeleteRecipe(sqlDB, recipe.ID); err != nil {
				return fmt.Errorf("deleting recipe: %w", err)
			}
			filter = db.RecipeFilter{}
			continue
		}
		if goRetry {
			_ = runRetryPipeline(recipe.ID)
			pendingDetailID = recipe.ID
			continue
		}
		if goManage {
			if err := runManageUI(); err != nil {
				return err
			}
			searchData, _ = loadSearchData()
			pendingDetailID = recipe.ID
			continue
		}
		if goPrint {
			quit, err := ui.RunPrintUI(recipe, export.Options{Credits: cfg.Credits})
			if err != nil {
				return err
			}
			if quit {
				return nil
			}
			pendingDetailID = recipe.ID
			continue
		}
		if goEdit {
			toSave, tagNames, _, err := ui.RunEditUI(recipe, editData)
			if err != nil {
				return err
			}
			if toSave != nil {
				if err := db.SaveRecipe(sqlDB, toSave, tagNames); err != nil {
					return fmt.Errorf("saving recipe: %w", err)
				}
				editData, _ = loadEditData()
				searchData, _ = loadSearchData()
			}
			// Re-open detail with the (possibly updated) recipe.
			pendingDetailID = recipe.ID
			continue
		}
		if goAdd {
			return runAdd(nil, nil)
		}
		if !goHome {
			break
		}

		// User chose "home" — loop back to the list, preserving advanced filters.
		filter.Query = searchQuery
	}

	return nil
}

func isTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}
